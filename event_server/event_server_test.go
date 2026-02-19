package event_server

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
)

// newTestServer creates an EventServer with an httptest.Server for use in tests.
// It returns the EventServer and a cleanup function.
func newTestServer(t *testing.T) (*EventServer, *httptest.Server, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	es := &EventServer{
		clients: make(map[SubscriberId]*Subscriber),
	}
	es.ctx, es.cancel = context.WithCancel(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/events", es.handleSSE)
	ts := httptest.NewServer(mux)

	cleanup := func() {
		cancel()
		ts.Close()
	}
	return es, ts, cleanup
}

func makeVehicleEvent(agencyID, routeID, vehicleID string) *pb.VehiclePositionEvent {
	return &pb.VehiclePositionEvent{
		Attributes: &pb.EventAttributes{
			AgencyId:  agencyID,
			RouteId:   routeID,
			VehicleId: vehicleID,
		},
		Latitude:  42.36,
		Longitude: -71.06,
	}
}

// TestSubscribeAddsClient verifies that Subscribe creates a subscriber and
// Unsubscribe removes it.
func TestSubscribeAddsClient(t *testing.T) {
	ctx := context.Background()
	es := &EventServer{clients: make(map[SubscriberId]*Subscriber)}
	es.ctx, es.cancel = context.WithCancel(ctx)

	sub := es.Subscribe(map[string]string{"agency_id": "mbta"})
	if sub == nil {
		t.Fatal("expected non-nil subscriber")
	}

	es.clientsMux.RLock()
	_, found := es.clients[sub.ID]
	es.clientsMux.RUnlock()
	if !found {
		t.Error("subscriber not found in client map after Subscribe")
	}

	es.Unsubscribe(sub)

	es.clientsMux.RLock()
	_, found = es.clients[sub.ID]
	es.clientsMux.RUnlock()
	if found {
		t.Error("subscriber still found in client map after Unsubscribe")
	}
}

// TestBroadcastDelivery verifies that Broadcast sends events to subscribers.
func TestBroadcastDelivery(t *testing.T) {
	ctx := context.Background()
	es := &EventServer{clients: make(map[SubscriberId]*Subscriber)}
	es.ctx, es.cancel = context.WithCancel(ctx)

	sub := es.Subscribe(map[string]string{})
	defer es.Unsubscribe(sub)

	event := makeVehicleEvent("mbta", "Red", "v1")

	go es.Broadcast(event)

	select {
	case msg := <-sub.Channel:
		eventMap, ok := msg.(map[string]any)
		if !ok {
			t.Fatalf("expected map[string]any, got %T", msg)
		}
		if eventMap["agency_id"] != "mbta" {
			t.Errorf("got agency_id %v, want %q", eventMap["agency_id"], "mbta")
		}
		if eventMap["route_id"] != "Red" {
			t.Errorf("got route_id %v, want %q", eventMap["route_id"], "Red")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for broadcasted event")
	}
}

// TestBroadcastFilteringByAgency verifies that only matching subscribers receive events.
func TestBroadcastFilteringByAgency(t *testing.T) {
	ctx := context.Background()
	es := &EventServer{clients: make(map[SubscriberId]*Subscriber)}
	es.ctx, es.cancel = context.WithCancel(ctx)

	subMBTA := es.Subscribe(map[string]string{"agency_id": "mbta"})
	defer es.Unsubscribe(subMBTA)

	subCTA := es.Subscribe(map[string]string{"agency_id": "cta"})
	defer es.Unsubscribe(subCTA)

	event := makeVehicleEvent("mbta", "Red", "v1")
	go es.Broadcast(event)

	// subMBTA should receive
	select {
	case msg := <-subMBTA.Channel:
		eventMap := msg.(map[string]any)
		if eventMap["agency_id"] != "mbta" {
			t.Errorf("mbta subscriber got wrong agency_id: %v", eventMap["agency_id"])
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout: mbta subscriber did not receive event")
	}

	// subCTA should NOT receive
	select {
	case msg := <-subCTA.Channel:
		t.Errorf("cta subscriber unexpectedly received event: %v", msg)
	case <-time.After(50 * time.Millisecond):
		// expected: no event for cta
	}
}

// TestBroadcastFilteringByRoute verifies route_id filtering.
func TestBroadcastFilteringByRoute(t *testing.T) {
	ctx := context.Background()
	es := &EventServer{clients: make(map[SubscriberId]*Subscriber)}
	es.ctx, es.cancel = context.WithCancel(ctx)

	subRed := es.Subscribe(map[string]string{"route_id": "Red"})
	defer es.Unsubscribe(subRed)

	subBlue := es.Subscribe(map[string]string{"route_id": "Blue"})
	defer es.Unsubscribe(subBlue)

	event := makeVehicleEvent("mbta", "Red", "v1")
	go es.Broadcast(event)

	select {
	case <-subRed.Channel:
		// expected
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout: Red subscriber did not receive event")
	}

	select {
	case msg := <-subBlue.Channel:
		t.Errorf("Blue subscriber unexpectedly received event: %v", msg)
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

// TestBroadcastNoSubscribers verifies Broadcast does not panic with no subscribers.
func TestBroadcastNoSubscribers(t *testing.T) {
	ctx := context.Background()
	es := &EventServer{clients: make(map[SubscriberId]*Subscriber)}
	es.ctx, es.cancel = context.WithCancel(ctx)

	// Should not panic
	es.Broadcast(makeVehicleEvent("mbta", "Red", "v1"))
}

// TestBroadcastMultipleMatchingSubscribers verifies all matching subscribers receive events.
func TestBroadcastMultipleMatchingSubscribers(t *testing.T) {
	ctx := context.Background()
	es := &EventServer{clients: make(map[SubscriberId]*Subscriber)}
	es.ctx, es.cancel = context.WithCancel(ctx)

	const n = 3
	subs := make([]*Subscriber, n)
	for i := range subs {
		subs[i] = es.Subscribe(map[string]string{}) // no filter → match all
		defer es.Unsubscribe(subs[i])
	}

	event := makeVehicleEvent("mbta", "Red", "v1")
	go es.Broadcast(event)

	for i, sub := range subs {
		select {
		case <-sub.Channel:
			// expected
		case <-time.After(200 * time.Millisecond):
			t.Errorf("timeout: subscriber %d did not receive event", i)
		}
	}
}

// TestSSEEndpointDeliversEvent tests the full SSE HTTP handler via httptest.
func TestSSEEndpointDeliversEvent(t *testing.T) {
	es, ts, cleanup := newTestServer(t)
	defer cleanup()

	// Subscribe via HTTP SSE
	url := fmt.Sprintf("%s/events", ts.URL)
	req, _ := http.NewRequest("GET", url, nil)

	// Use a context with timeout to auto-cancel the SSE connection
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	// Start SSE client in background
	received := make(chan string, 1)
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				received <- strings.TrimPrefix(line, "data: ")
				return
			}
		}
	}()

	// Wait for the SSE client to register
	time.Sleep(50 * time.Millisecond)

	// Broadcast an event
	event := makeVehicleEvent("mbta", "Red", "v1")
	es.Broadcast(event)

	select {
	case data := <-received:
		if !strings.Contains(data, "mbta") {
			t.Errorf("SSE data missing agency_id 'mbta': %s", data)
		}
	case <-time.After(400 * time.Millisecond):
		t.Fatal("timeout waiting for SSE event")
	}
}

// TestSSEEndpointFiltersByQueryParam tests SSE filtering via query parameters.
func TestSSEEndpointFiltersByQueryParam(t *testing.T) {
	es, ts, cleanup := newTestServer(t)
	defer cleanup()

	// SSE client filtered to agency_id=mbta only
	url := fmt.Sprintf("%s/events?agency_id=mbta", ts.URL)
	req, _ := http.NewRequest("GET", url, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	received := make(chan string, 2)
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				received <- strings.TrimPrefix(line, "data: ")
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)

	// Broadcast non-matching event (cta) - should NOT arrive
	es.Broadcast(makeVehicleEvent("cta", "Blue", "v2"))

	// Small pause to ensure non-matching event is not delivered
	time.Sleep(30 * time.Millisecond)

	// Broadcast matching event (mbta) - should arrive
	es.Broadcast(makeVehicleEvent("mbta", "Red", "v1"))

	select {
	case data := <-received:
		if !strings.Contains(data, "mbta") {
			t.Errorf("SSE data missing agency_id 'mbta': %s", data)
		}
	case <-time.After(400 * time.Millisecond):
		t.Fatal("timeout: did not receive matching SSE event for mbta")
	}

	// No second event should have arrived (cta was filtered)
	select {
	case data := <-received:
		t.Errorf("unexpected second SSE event (should have been filtered): %s", data)
	case <-time.After(50 * time.Millisecond):
		// expected: only mbta event came through
	}
}

// TestSSEClientDisconnectCleanup verifies the subscriber is removed when the client disconnects.
func TestSSEClientDisconnectCleanup(t *testing.T) {
	es, ts, cleanup := newTestServer(t)
	defer cleanup()

	url := fmt.Sprintf("%s/events", ts.URL)
	req, _ := http.NewRequest("GET", url, nil)
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)

	connDone := make(chan struct{})
	go func() {
		defer close(connDone)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		resp.Body.Close()
	}()

	// Wait for client to register
	time.Sleep(50 * time.Millisecond)

	es.clientsMux.RLock()
	countBefore := len(es.clients)
	es.clientsMux.RUnlock()

	if countBefore != 1 {
		t.Fatalf("expected 1 subscriber, got %d", countBefore)
	}

	// Disconnect the client
	cancel()
	<-connDone

	// Give the handler time to call Unsubscribe
	time.Sleep(50 * time.Millisecond)

	es.clientsMux.RLock()
	countAfter := len(es.clients)
	es.clientsMux.RUnlock()

	if countAfter != 0 {
		t.Errorf("expected 0 subscribers after disconnect, got %d", countAfter)
	}
}

// TestNewSubscriberHasUniqueIDs verifies each subscriber gets a unique ID.
func TestNewSubscriberHasUniqueIDs(t *testing.T) {
	ctx := context.Background()
	es := &EventServer{clients: make(map[SubscriberId]*Subscriber)}
	es.ctx, es.cancel = context.WithCancel(ctx)

	sub1 := es.Subscribe(map[string]string{})
	sub2 := es.Subscribe(map[string]string{})
	defer es.Unsubscribe(sub1)
	defer es.Unsubscribe(sub2)

	if sub1.ID == sub2.ID {
		t.Error("expected unique subscriber IDs, got the same ID for both")
	}
}

// TestEventServerConfig verifies that NewEventServer can be constructed with a config.
func TestEventServerConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use port 0 to let the OS pick a free port; we just verify construction doesn't panic.
	// Note: port "0" may not be supported by all HTTP servers, so we use a high port.
	// We test config wiring only and immediately cancel.
	cfg := config.EventServerConfig{Port: "0", Path: "/events"}
	es := NewEventServer(ctx, cfg)
	if es == nil {
		t.Fatal("expected non-nil EventServer")
	}
}
