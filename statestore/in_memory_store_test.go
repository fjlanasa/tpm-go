package statestore

import (
	"sync"
	"testing"
	"time"

	"github.com/fjlanasa/tpm-go/config"
	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"google.golang.org/protobuf/proto"
)

func newTestStore(expiry time.Duration) *InMemoryStateStore {
	return NewInMemoryStateStore(config.InMemoryStateStoreConfig{
		Expiry: expiry,
	})
}

func newVehiclePositionEvent() proto.Message {
	return &pb.VehiclePositionEvent{}
}

func TestGetSetBasic(t *testing.T) {
	store := newTestStore(time.Hour)
	defer store.Close()

	msg := &pb.VehiclePositionEvent{
		Attributes: &pb.EventAttributes{
			VehicleId: "v1",
			StopId:    "s1",
		},
	}

	if err := store.Set("key1", msg, 0); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	got, found := store.Get("key1", newVehiclePositionEvent)
	if !found {
		t.Fatal("expected key1 to be found")
	}

	vp, ok := got.(*pb.VehiclePositionEvent)
	if !ok {
		t.Fatal("expected *pb.VehiclePositionEvent")
	}
	if vp.GetAttributes().GetVehicleId() != "v1" {
		t.Errorf("got vehicle_id %q, want %q", vp.GetAttributes().GetVehicleId(), "v1")
	}
	if vp.GetAttributes().GetStopId() != "s1" {
		t.Errorf("got stop_id %q, want %q", vp.GetAttributes().GetStopId(), "s1")
	}
}

func TestGetMissing(t *testing.T) {
	store := newTestStore(time.Hour)
	defer store.Close()

	got, found := store.Get("nonexistent", newVehiclePositionEvent)
	if found {
		t.Error("expected found=false for missing key")
	}
	if got == nil {
		t.Error("expected non-nil default message from factory")
	}
}

func TestGetMissingWithNilFactory(t *testing.T) {
	store := newTestStore(time.Hour)
	defer store.Close()

	got, found := store.Get("nonexistent", func() proto.Message { return nil })
	if found {
		t.Error("expected found=false for missing key")
	}
	if got != nil {
		t.Error("expected nil when factory returns nil")
	}
}

func TestDelete(t *testing.T) {
	store := newTestStore(time.Hour)
	defer store.Close()

	msg := &pb.VehiclePositionEvent{
		Attributes: &pb.EventAttributes{VehicleId: "v1"},
	}
	_ = store.Set("key1", msg, 0)

	store.Delete("key1")

	_, found := store.Get("key1", newVehiclePositionEvent)
	if found {
		t.Error("expected key1 to be deleted")
	}
}

func TestDeleteNonexistent(t *testing.T) {
	store := newTestStore(time.Hour)
	defer store.Close()

	// Should not panic
	store.Delete("nonexistent")
}

func TestSetOverwrite(t *testing.T) {
	store := newTestStore(time.Hour)
	defer store.Close()

	msg1 := &pb.VehiclePositionEvent{
		Attributes: &pb.EventAttributes{VehicleId: "v1"},
	}
	msg2 := &pb.VehiclePositionEvent{
		Attributes: &pb.EventAttributes{VehicleId: "v2"},
	}

	_ = store.Set("key1", msg1, 0)
	_ = store.Set("key1", msg2, 0)

	got, found := store.Get("key1", newVehiclePositionEvent)
	if !found {
		t.Fatal("expected key1 to be found")
	}
	vp := got.(*pb.VehiclePositionEvent)
	if vp.GetAttributes().GetVehicleId() != "v2" {
		t.Errorf("got vehicle_id %q, want %q", vp.GetAttributes().GetVehicleId(), "v2")
	}
}

func TestTTLExpiration(t *testing.T) {
	store := newTestStore(50 * time.Millisecond)
	defer store.Close()

	msg := &pb.VehiclePositionEvent{
		Attributes: &pb.EventAttributes{VehicleId: "v1"},
	}
	_ = store.Set("key1", msg, 50*time.Millisecond)

	// Should be found immediately
	_, found := store.Get("key1", newVehiclePositionEvent)
	if !found {
		t.Fatal("expected key1 to be found before expiration")
	}

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Should no longer be found (expired per Get's check)
	_, found = store.Get("key1", newVehiclePositionEvent)
	if found {
		t.Error("expected key1 to be expired")
	}
}

func TestCustomTTLOverridesDefault(t *testing.T) {
	store := newTestStore(time.Hour)
	defer store.Close()

	msg := &pb.VehiclePositionEvent{
		Attributes: &pb.EventAttributes{VehicleId: "v1"},
	}
	_ = store.Set("key1", msg, 50*time.Millisecond)

	time.Sleep(100 * time.Millisecond)

	_, found := store.Get("key1", newVehiclePositionEvent)
	if found {
		t.Error("expected key1 to be expired with custom TTL")
	}
}

func TestZeroTTLUsesDefault(t *testing.T) {
	store := newTestStore(time.Hour)
	defer store.Close()

	msg := &pb.VehiclePositionEvent{
		Attributes: &pb.EventAttributes{VehicleId: "v1"},
	}
	// TTL=0 should use the store's default (1 hour)
	_ = store.Set("key1", msg, 0)

	// Should still be present after a short wait
	time.Sleep(50 * time.Millisecond)
	_, found := store.Get("key1", newVehiclePositionEvent)
	if !found {
		t.Error("expected key1 to still be present with default TTL")
	}
}

func TestDefaultExpiry(t *testing.T) {
	// When Expiry is 0 in config, NewInMemoryStateStore defaults to 1 hour
	store := NewInMemoryStateStore(config.InMemoryStateStoreConfig{Expiry: 0})
	defer store.Close()

	if store.ttl != time.Hour {
		t.Errorf("expected default TTL of 1h, got %v", store.ttl)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	store := newTestStore(time.Hour)
	defer store.Close()

	var wg sync.WaitGroup
	const numGoroutines = 50

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			msg := &pb.VehiclePositionEvent{
				Attributes: &pb.EventAttributes{
					VehicleId: "v1",
				},
			}
			_ = store.Set("key1", msg, 0)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Get("key1", newVehiclePositionEvent)
		}()
	}

	// Concurrent deletes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Delete("key1")
		}()
	}

	wg.Wait()
}

func TestCloseStopsCleanupGoroutine(t *testing.T) {
	store := newTestStore(50 * time.Millisecond)

	msg := &pb.VehiclePositionEvent{
		Attributes: &pb.EventAttributes{VehicleId: "v1"},
	}
	_ = store.Set("key1", msg, 50*time.Millisecond)

	// Close should not panic and should stop the goroutine
	store.Close()

	// Give the goroutine time to exit
	time.Sleep(100 * time.Millisecond)

	// The store itself should still be usable for Get (though the background
	// cleanup is stopped). The expiration check in Get still works inline.
	_, found := store.Get("key1", newVehiclePositionEvent)
	if found {
		t.Error("expected key1 to be expired even after Close")
	}
}

func TestMultipleKeys(t *testing.T) {
	store := newTestStore(time.Hour)
	defer store.Close()

	msg1 := &pb.VehiclePositionEvent{
		Attributes: &pb.EventAttributes{VehicleId: "v1"},
	}
	msg2 := &pb.VehiclePositionEvent{
		Attributes: &pb.EventAttributes{VehicleId: "v2"},
	}
	msg3 := &pb.VehiclePositionEvent{
		Attributes: &pb.EventAttributes{VehicleId: "v3"},
	}

	_ = store.Set("key1", msg1, 0)
	_ = store.Set("key2", msg2, 0)
	_ = store.Set("key3", msg3, 0)

	for _, tc := range []struct {
		key       string
		vehicleID string
	}{
		{"key1", "v1"},
		{"key2", "v2"},
		{"key3", "v3"},
	} {
		got, found := store.Get(tc.key, newVehiclePositionEvent)
		if !found {
			t.Errorf("expected %s to be found", tc.key)
			continue
		}
		vp := got.(*pb.VehiclePositionEvent)
		if vp.GetAttributes().GetVehicleId() != tc.vehicleID {
			t.Errorf("key %s: got vehicle_id %q, want %q", tc.key, vp.GetAttributes().GetVehicleId(), tc.vehicleID)
		}
	}

	// Delete one, verify others remain
	store.Delete("key2")
	_, found := store.Get("key2", newVehiclePositionEvent)
	if found {
		t.Error("expected key2 to be deleted")
	}
	_, found = store.Get("key1", newVehiclePositionEvent)
	if !found {
		t.Error("expected key1 to still exist")
	}
	_, found = store.Get("key3", newVehiclePositionEvent)
	if !found {
		t.Error("expected key3 to still exist")
	}
}

func TestGetWithNewFactory(t *testing.T) {
	store := newTestStore(time.Hour)
	defer store.Close()

	// When key is missing, the factory function should create a fresh message
	factory := func() proto.Message {
		return &pb.StopEvent{
			StopEventType: pb.StopEvent_UNKNOWN,
		}
	}

	got, found := store.Get("missing", factory)
	if found {
		t.Error("expected found=false for missing key")
	}
	if got == nil {
		t.Fatal("expected non-nil message from factory")
	}

	stopEvent, ok := got.(*pb.StopEvent)
	if !ok {
		t.Fatal("expected *pb.StopEvent from factory")
	}
	if stopEvent.GetStopEventType() != pb.StopEvent_UNKNOWN {
		t.Errorf("expected UNKNOWN event type, got %v", stopEvent.GetStopEventType())
	}
}
