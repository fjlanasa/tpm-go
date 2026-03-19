package graphs

import (
	"context"
	"testing"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
	"google.golang.org/protobuf/proto"
)

// buildFeed creates a GTFS-RT FeedMessage containing the given vehicle snapshots.
func buildFeed(vehicles []vehicleSnapshot) []byte {
	entities := make([]*gtfs.FeedEntity, len(vehicles))
	for i, v := range vehicles {
		status := gtfs.VehiclePosition_VehicleStopStatus(v.status)
		stopSeq := uint32(v.stopSequence)
		entities[i] = &gtfs.FeedEntity{
			Id: proto.String(v.vehicleID),
			Vehicle: &gtfs.VehiclePosition{
				Vehicle:              &gtfs.VehicleDescriptor{Id: proto.String(v.vehicleID)},
				Trip:                 &gtfs.TripDescriptor{RouteId: proto.String(v.routeID), TripId: proto.String(v.tripID), DirectionId: proto.Uint32(0), StartDate: proto.String("20240101")},
				CurrentStatus:        &status,
				StopId:               proto.String(v.stopID),
				CurrentStopSequence:  &stopSeq,
				Timestamp:            proto.Uint64(v.timestamp),
				Position:             &gtfs.Position{Latitude: proto.Float32(42.35), Longitude: proto.Float32(-71.06)},
			},
		}
	}
	data, _ := proto.Marshal(&gtfs.FeedMessage{
		Header: &gtfs.FeedHeader{
			GtfsRealtimeVersion: proto.String("2.0"),
			Timestamp:           proto.Uint64(vehicles[0].timestamp),
		},
		Entity: entities,
	})
	return data
}

type vehicleSnapshot struct {
	vehicleID    string
	stopID       string
	routeID      string
	tripID       string
	status       int32
	stopSequence int
	timestamp    uint64
}

// fullGraphConfig returns a config wiring all 6 pipeline types with connectors.
// The entry point is the "input" connector; terminal outputs go to
// "se-out", "headway-out", "dwell-out", and "travel-out".
func fullGraphConfig() config.GraphConfig {
	return config.GraphConfig{
		Sources: map[config.ID]config.SourceConfig{},
		Sinks:   map[config.ID]config.SinkConfig{},
		Connectors: map[config.ID]any{
			"input":         nil,
			"fm-to-vp":      nil,
			"vp-to-se":      nil,
			"se-to-headway": nil,
			"se-to-dwell":   nil,
			"se-to-travel":  nil,
			"se-out":        nil,
			"headway-out":   nil,
			"dwell-out":     nil,
			"travel-out":    nil,
		},
		StateStores: map[config.ID]config.StateStoreConfig{
			"se-store":      {Type: config.InMemoryStateStoreType, InMemory: config.InMemoryStateStoreConfig{Expiry: time.Hour}},
			"headway-store": {Type: config.InMemoryStateStoreType, InMemory: config.InMemoryStateStoreConfig{Expiry: time.Hour}},
			"dwell-store":   {Type: config.InMemoryStateStoreType, InMemory: config.InMemoryStateStoreConfig{Expiry: time.Hour}},
			"travel-store":  {Type: config.InMemoryStateStoreType, InMemory: config.InMemoryStateStoreConfig{Expiry: time.Hour}},
		},
		Pipelines: map[config.ID]config.PipelineConfig{
			"fm-pipe": {
				ID: "fm-pipe", AgencyID: "test-agency", Type: config.PipelineTypeFeedMessage,
				Sources: []config.ID{"input"}, Sinks: []config.ID{"fm-to-vp"},
			},
			"vp-pipe": {
				ID: "vp-pipe", Type: config.PipelineTypeVehiclePosition,
				Sources: []config.ID{"fm-to-vp"}, Sinks: []config.ID{"vp-to-se"},
			},
			"se-pipe": {
				ID: "se-pipe", Type: config.PipelineTypeStopEvent, StateStore: "se-store",
				Sources: []config.ID{"vp-to-se"}, Sinks: []config.ID{"se-to-headway", "se-to-dwell", "se-to-travel", "se-out"},
			},
			"headway-pipe": {
				ID: "headway-pipe", Type: config.PipelineTypeHeadwayEvent, StateStore: "headway-store",
				Sources: []config.ID{"se-to-headway"}, Sinks: []config.ID{"headway-out"},
			},
			"dwell-pipe": {
				ID: "dwell-pipe", Type: config.PipelineTypeDwellEvent, StateStore: "dwell-store",
				Sources: []config.ID{"se-to-dwell"}, Sinks: []config.ID{"dwell-out"},
			},
			"travel-pipe": {
				ID: "travel-pipe", Type: config.PipelineTypeTravelTime, StateStore: "travel-store",
				Sources: []config.ID{"se-to-travel"}, Sinks: []config.ID{"travel-out"},
			},
		},
	}
}

// collect drains a channel into a slice until the channel closes or the
// context is cancelled. Returns collected items.
func collect[T any](ctx context.Context, ch <-chan any) []T {
	var results []T
	for {
		select {
		case <-ctx.Done():
			return results
		case item, ok := <-ch:
			if !ok {
				return results
			}
			if typed, ok := item.(T); ok {
				results = append(results, typed)
			}
		}
	}
}

// TestGraphEndToEnd feeds GTFS-RT snapshots through the full 6-pipeline
// graph and verifies that stop events, dwell times, headways, and travel
// times are produced correctly.
func TestGraphEndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := fullGraphConfig()

	// Capture connector channels so we can inject input and read output.
	var connectors map[config.ID]chan any
	captureConnectors := func(_ *Graph, c *map[config.ID]chan any, _ *config.GraphConfig) {
		connectors = *c
	}

	graph, err := NewGraph(ctx, cfg, captureConnectors)
	if err != nil {
		t.Fatalf("NewGraph: %v", err)
	}
	defer graph.Close()
	graph.Run()

	// Start collectors before sending data so output channels don't block.
	type results struct {
		stopEvents  []*pb.StopEvent
		dwellEvents []*pb.DwellTimeEvent
		headways    []*pb.HeadwayTimeEvent
		travelTimes []*pb.TravelTimeEvent
	}
	var r results
	seDone := make(chan struct{})
	dwDone := make(chan struct{})
	hwDone := make(chan struct{})
	ttDone := make(chan struct{})

	go func() { r.stopEvents = collect[*pb.StopEvent](ctx, connectors["se-out"]); close(seDone) }()
	go func() { r.dwellEvents = collect[*pb.DwellTimeEvent](ctx, connectors["dwell-out"]); close(dwDone) }()
	go func() { r.headways = collect[*pb.HeadwayTimeEvent](ctx, connectors["headway-out"]); close(hwDone) }()
	go func() { r.travelTimes = collect[*pb.TravelTimeEvent](ctx, connectors["travel-out"]); close(ttDone) }()

	// Scenario: two vehicles on the Red line.
	//
	// Feed 1 (t=1000): v1 IN_TRANSIT_TO s1, v2 STOPPED_AT s1     (initial state)
	// Feed 2 (t=1010): v1 STOPPED_AT s1,    v2 STOPPED_AT s1     → v1 ARRIVAL@s1
	// Feed 3 (t=1030): v1 IN_TRANSIT_TO s2, v2 STOPPED_AT s1     → v1 DEPARTURE@s1 → dwell(v1,s1)=20s
	// Feed 4 (t=1060): v1 STOPPED_AT s2,    v2 IN_TRANSIT_TO s2  → v1 ARRIVAL@s2, v2 DEPARTURE@s1
	//                                                               → travel(v1,s1→s2)=50s, headway(s1)=30s
	feeds := [][]byte{
		buildFeed([]vehicleSnapshot{
			{vehicleID: "v1", stopID: "s1", routeID: "Red", tripID: "t1", status: int32(gtfs.VehiclePosition_IN_TRANSIT_TO), stopSequence: 5, timestamp: 1000},
			{vehicleID: "v2", stopID: "s1", routeID: "Red", tripID: "t2", status: int32(gtfs.VehiclePosition_STOPPED_AT), stopSequence: 5, timestamp: 1000},
		}),
		buildFeed([]vehicleSnapshot{
			{vehicleID: "v1", stopID: "s1", routeID: "Red", tripID: "t1", status: int32(gtfs.VehiclePosition_STOPPED_AT), stopSequence: 5, timestamp: 1010},
			{vehicleID: "v2", stopID: "s1", routeID: "Red", tripID: "t2", status: int32(gtfs.VehiclePosition_STOPPED_AT), stopSequence: 5, timestamp: 1010},
		}),
		buildFeed([]vehicleSnapshot{
			{vehicleID: "v1", stopID: "s2", routeID: "Red", tripID: "t1", status: int32(gtfs.VehiclePosition_IN_TRANSIT_TO), stopSequence: 6, timestamp: 1030},
			{vehicleID: "v2", stopID: "s1", routeID: "Red", tripID: "t2", status: int32(gtfs.VehiclePosition_STOPPED_AT), stopSequence: 5, timestamp: 1030},
		}),
		buildFeed([]vehicleSnapshot{
			{vehicleID: "v1", stopID: "s2", routeID: "Red", tripID: "t1", status: int32(gtfs.VehiclePosition_STOPPED_AT), stopSequence: 6, timestamp: 1060},
			{vehicleID: "v2", stopID: "s2", routeID: "Red", tripID: "t2", status: int32(gtfs.VehiclePosition_IN_TRANSIT_TO), stopSequence: 6, timestamp: 1060},
		}),
	}

	input := connectors["input"]
	for _, feed := range feeds {
		input <- feed
	}
	close(input)

	// Wait for all collectors to finish (channels close when pipelines drain).
	<-seDone
	<-dwDone
	<-hwDone
	<-ttDone

	// --- Stop events ---
	// Expected: v1 ARRIVAL@s1, v1 DEPARTURE@s1, v2 DEPARTURE@s1, v1 ARRIVAL@s2
	if len(r.stopEvents) != 4 {
		t.Fatalf("stop events: got %d, want 4", len(r.stopEvents))
	}

	// Build a set of (vehicleID, stopID, eventType) for flexible ordering.
	type seKey struct {
		vehicle string
		stop    string
		typ     pb.StopEvent_EventType
	}
	seSet := map[seKey]bool{}
	for _, se := range r.stopEvents {
		seSet[seKey{se.GetAttributes().GetVehicleId(), se.GetAttributes().GetStopId(), se.GetStopEventType()}] = true
	}
	for _, want := range []seKey{
		{"v1", "s1", pb.StopEvent_ARRIVAL},
		{"v1", "s1", pb.StopEvent_DEPARTURE},
		{"v2", "s1", pb.StopEvent_DEPARTURE},
		{"v1", "s2", pb.StopEvent_ARRIVAL},
	} {
		if !seSet[want] {
			t.Errorf("missing stop event: vehicle=%s stop=%s type=%s", want.vehicle, want.stop, want.typ)
		}
	}

	// Verify agency propagation.
	for _, se := range r.stopEvents {
		if se.GetAttributes().GetAgencyId() != "test-agency" {
			t.Errorf("stop event agency: got %q, want %q", se.GetAttributes().GetAgencyId(), "test-agency")
		}
	}

	// --- Dwell time ---
	// Expected: v1 at s1, dwell = 20s (arrival t=1010, departure t=1030)
	if len(r.dwellEvents) != 1 {
		t.Fatalf("dwell events: got %d, want 1", len(r.dwellEvents))
	}
	dw := r.dwellEvents[0]
	if dw.GetAttributes().GetVehicleId() != "v1" || dw.GetAttributes().GetStopId() != "s1" {
		t.Errorf("dwell: got vehicle=%q stop=%q, want v1 s1",
			dw.GetAttributes().GetVehicleId(), dw.GetAttributes().GetStopId())
	}
	if dw.GetDwellTimeSeconds() != 20 {
		t.Errorf("dwell seconds: got %d, want 20", dw.GetDwellTimeSeconds())
	}

	// --- Headway ---
	// Expected: at s1, v2 departs 30s after v1 departs
	if len(r.headways) != 1 {
		t.Fatalf("headway events: got %d, want 1", len(r.headways))
	}
	hw := r.headways[0]
	if hw.GetLeadingVehicleId() != "v1" || hw.GetFollowingVehicleId() != "v2" {
		t.Errorf("headway: got leading=%q following=%q, want v1 v2",
			hw.GetLeadingVehicleId(), hw.GetFollowingVehicleId())
	}
	if hw.GetHeadwaySeconds() != 30 {
		t.Errorf("headway seconds: got %d, want 30", hw.GetHeadwaySeconds())
	}
	if hw.GetAttributes().GetStopId() != "s1" {
		t.Errorf("headway stop: got %q, want s1", hw.GetAttributes().GetStopId())
	}

	// --- Travel time ---
	// Expected: v1 from s1 to s2, travel = 50s (arrival s1 t=1010, arrival s2 t=1060)
	if len(r.travelTimes) != 1 {
		t.Fatalf("travel time events: got %d, want 1", len(r.travelTimes))
	}
	tt := r.travelTimes[0]
	if tt.GetAttributes().GetVehicleId() != "v1" {
		t.Errorf("travel time vehicle: got %q, want v1", tt.GetAttributes().GetVehicleId())
	}
	if tt.GetAttributes().GetOriginStopId() != "s1" || tt.GetAttributes().GetDestinationStopId() != "s2" {
		t.Errorf("travel time stops: got %q→%q, want s1→s2",
			tt.GetAttributes().GetOriginStopId(), tt.GetAttributes().GetDestinationStopId())
	}
	if tt.GetTravelTimeSeconds() != 50 {
		t.Errorf("travel time seconds: got %d, want 50", tt.GetTravelTimeSeconds())
	}
}
