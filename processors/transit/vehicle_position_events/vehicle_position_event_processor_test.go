package processors

import (
	"context"
	"testing"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"google.golang.org/protobuf/proto"
)

func createFeedMessageEvent(agencyID string, entities ...*gtfs.FeedEntity) *pb.FeedMessageEvent {
	return &pb.FeedMessageEvent{
		AgencyId: agencyID,
		FeedMessage: &gtfs.FeedMessage{
			Header: &gtfs.FeedHeader{
				GtfsRealtimeVersion: proto.String("2.0"),
				Timestamp:           proto.Uint64(uint64(time.Now().Unix())),
			},
			Entity: entities,
		},
	}
}

func createVehicleEntity(entityID, vehicleID, routeID, tripID, stopID string, status gtfs.VehiclePosition_VehicleStopStatus, timestamp uint64, lat, lon float32, stopSeq uint32, directionID uint32, startDate string) *gtfs.FeedEntity {
	return &gtfs.FeedEntity{
		Id: proto.String(entityID),
		Vehicle: &gtfs.VehiclePosition{
			Vehicle: &gtfs.VehicleDescriptor{
				Id: proto.String(vehicleID),
			},
			Trip: &gtfs.TripDescriptor{
				RouteId:     proto.String(routeID),
				TripId:      proto.String(tripID),
				DirectionId: proto.Uint32(directionID),
				StartDate:   proto.String(startDate),
			},
			StopId:              proto.String(stopID),
			CurrentStopSequence: proto.Uint32(stopSeq),
			CurrentStatus:       &status,
			Timestamp:           proto.Uint64(timestamp),
			Position: &gtfs.Position{
				Latitude:  proto.Float32(lat),
				Longitude: proto.Float32(lon),
			},
		},
	}
}

// collectResults collects VehiclePositionEvents from the processor's output channel
// using a timeout to determine when processing is complete. This is necessary because
// VehiclePositionEventProcessor.doStream does not close its output channel on input close.
func collectResults(t *testing.T, out <-chan any, expected int) []*pb.VehiclePositionEvent {
	t.Helper()
	results := make([]*pb.VehiclePositionEvent, 0, expected)
	timeout := time.After(500 * time.Millisecond)
	for {
		select {
		case event := <-out:
			if vpe, ok := event.(*pb.VehiclePositionEvent); ok {
				results = append(results, vpe)
				if len(results) == expected {
					return results
				}
			}
		case <-timeout:
			return results
		}
	}
}

func TestVehiclePositionEventProcessor(t *testing.T) {
	tests := []struct {
		name     string
		input    *pb.FeedMessageEvent
		expected []*pb.VehiclePositionEvent
	}{
		{
			name: "extracts single vehicle position",
			input: createFeedMessageEvent("mbta",
				createVehicleEntity("e1", "v1", "Red", "trip1", "s1",
					gtfs.VehiclePosition_IN_TRANSIT_TO, 1000, 42.35, -71.06, 5, 0, "20240101"),
			),
			expected: []*pb.VehiclePositionEvent{
				{
					Attributes: &pb.EventAttributes{
						AgencyId:     "mbta",
						VehicleId:    "v1",
						RouteId:      "Red",
						TripId:       "trip1",
						StopId:       "s1",
						DirectionId:  "0",
						StopSequence: 5,
						StopStatus:   pb.StopStatus_IN_TRANSIT_TO,
						ServiceDate:  "20240101",
					},
					Latitude:  42.35,
					Longitude: -71.06,
				},
			},
		},
		{
			name: "extracts multiple vehicle positions",
			input: createFeedMessageEvent("mbta",
				createVehicleEntity("e1", "v1", "Red", "trip1", "s1",
					gtfs.VehiclePosition_IN_TRANSIT_TO, 1000, 42.35, -71.06, 5, 0, "20240101"),
				createVehicleEntity("e2", "v2", "Blue", "trip2", "s2",
					gtfs.VehiclePosition_STOPPED_AT, 1001, 42.36, -71.07, 10, 1, "20240101"),
				createVehicleEntity("e3", "v3", "Green", "trip3", "s3",
					gtfs.VehiclePosition_INCOMING_AT, 1002, 42.37, -71.08, 15, 0, "20240101"),
			),
			expected: []*pb.VehiclePositionEvent{
				{
					Attributes: &pb.EventAttributes{
						AgencyId:     "mbta",
						VehicleId:    "v1",
						RouteId:      "Red",
						TripId:       "trip1",
						StopId:       "s1",
						DirectionId:  "0",
						StopSequence: 5,
						StopStatus:   pb.StopStatus_IN_TRANSIT_TO,
						ServiceDate:  "20240101",
					},
					Latitude:  42.35,
					Longitude: -71.06,
				},
				{
					Attributes: &pb.EventAttributes{
						AgencyId:     "mbta",
						VehicleId:    "v2",
						RouteId:      "Blue",
						TripId:       "trip2",
						StopId:       "s2",
						DirectionId:  "1",
						StopSequence: 10,
						StopStatus:   pb.StopStatus_STOPPED_AT,
						ServiceDate:  "20240101",
					},
					Latitude:  42.36,
					Longitude: -71.07,
				},
				{
					Attributes: &pb.EventAttributes{
						AgencyId:     "mbta",
						VehicleId:    "v3",
						RouteId:      "Green",
						TripId:       "trip3",
						StopId:       "s3",
						DirectionId:  "0",
						StopSequence: 15,
						StopStatus:   pb.StopStatus_INCOMING_AT,
						ServiceDate:  "20240101",
					},
					Latitude:  42.37,
					Longitude: -71.08,
				},
			},
		},
		{
			name:     "empty feed produces no events",
			input:    createFeedMessageEvent("mbta"),
			expected: []*pb.VehiclePositionEvent{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			flow := NewVehiclePositionEventProcessor(ctx)

			flow.In() <- tt.input

			results := collectResults(t, flow.Out(), len(tt.expected))

			if len(results) != len(tt.expected) {
				t.Fatalf("got %d events, want %d", len(results), len(tt.expected))
			}

			for i, want := range tt.expected {
				got := results[i]
				if got.GetAttributes().GetAgencyId() != want.GetAttributes().GetAgencyId() {
					t.Errorf("event %d: got agency_id %q, want %q", i, got.GetAttributes().GetAgencyId(), want.GetAttributes().GetAgencyId())
				}
				if got.GetAttributes().GetVehicleId() != want.GetAttributes().GetVehicleId() {
					t.Errorf("event %d: got vehicle_id %q, want %q", i, got.GetAttributes().GetVehicleId(), want.GetAttributes().GetVehicleId())
				}
				if got.GetAttributes().GetRouteId() != want.GetAttributes().GetRouteId() {
					t.Errorf("event %d: got route_id %q, want %q", i, got.GetAttributes().GetRouteId(), want.GetAttributes().GetRouteId())
				}
				if got.GetAttributes().GetTripId() != want.GetAttributes().GetTripId() {
					t.Errorf("event %d: got trip_id %q, want %q", i, got.GetAttributes().GetTripId(), want.GetAttributes().GetTripId())
				}
				if got.GetAttributes().GetStopId() != want.GetAttributes().GetStopId() {
					t.Errorf("event %d: got stop_id %q, want %q", i, got.GetAttributes().GetStopId(), want.GetAttributes().GetStopId())
				}
				if got.GetAttributes().GetDirectionId() != want.GetAttributes().GetDirectionId() {
					t.Errorf("event %d: got direction_id %q, want %q", i, got.GetAttributes().GetDirectionId(), want.GetAttributes().GetDirectionId())
				}
				if got.GetAttributes().GetStopSequence() != want.GetAttributes().GetStopSequence() {
					t.Errorf("event %d: got stop_sequence %d, want %d", i, got.GetAttributes().GetStopSequence(), want.GetAttributes().GetStopSequence())
				}
				if got.GetAttributes().GetStopStatus() != want.GetAttributes().GetStopStatus() {
					t.Errorf("event %d: got stop_status %v, want %v", i, got.GetAttributes().GetStopStatus(), want.GetAttributes().GetStopStatus())
				}
				if got.GetAttributes().GetServiceDate() != want.GetAttributes().GetServiceDate() {
					t.Errorf("event %d: got service_date %q, want %q", i, got.GetAttributes().GetServiceDate(), want.GetAttributes().GetServiceDate())
				}
			}
		})
	}
}

func TestVehiclePositionEventProcessorStatusMapping(t *testing.T) {
	tests := []struct {
		name           string
		gtfsStatus     gtfs.VehiclePosition_VehicleStopStatus
		expectedStatus pb.StopStatus
	}{
		{"IN_TRANSIT_TO maps correctly", gtfs.VehiclePosition_IN_TRANSIT_TO, pb.StopStatus_IN_TRANSIT_TO},
		{"STOPPED_AT maps correctly", gtfs.VehiclePosition_STOPPED_AT, pb.StopStatus_STOPPED_AT},
		{"INCOMING_AT maps correctly", gtfs.VehiclePosition_INCOMING_AT, pb.StopStatus_INCOMING_AT},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			flow := NewVehiclePositionEventProcessor(ctx)

			input := createFeedMessageEvent("mbta",
				createVehicleEntity("e1", "v1", "Red", "trip1", "s1",
					tt.gtfsStatus, 1000, 42.35, -71.06, 1, 0, "20240101"),
			)
			flow.In() <- input

			results := collectResults(t, flow.Out(), 1)

			if len(results) != 1 {
				t.Fatal("expected 1 output event")
			}
			if results[0].GetAttributes().GetStopStatus() != tt.expectedStatus {
				t.Errorf("got status %v, want %v", results[0].GetAttributes().GetStopStatus(), tt.expectedStatus)
			}
		})
	}
}

func TestVehiclePositionEventProcessorNonFeedMessageIgnored(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flow := NewVehiclePositionEventProcessor(ctx)

	// Send non-FeedMessageEvent types
	flow.In() <- "not a feed message"
	flow.In() <- 42
	flow.In() <- &pb.VehiclePositionEvent{}

	results := collectResults(t, flow.Out(), 0)

	if len(results) != 0 {
		t.Errorf("expected 0 events for non-FeedMessageEvent input, got %d", len(results))
	}
}

func TestVehiclePositionEventProcessorVehicleIDFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flow := NewVehiclePositionEventProcessor(ctx)

	entity := &gtfs.FeedEntity{
		Id: proto.String("e1"),
		Vehicle: &gtfs.VehiclePosition{
			Vehicle: &gtfs.VehicleDescriptor{
				Id: proto.String(""), // empty vehicle ID
			},
			Trip: &gtfs.TripDescriptor{
				RouteId:     proto.String("Red"),
				TripId:      proto.String("trip-fallback"),
				DirectionId: proto.Uint32(0),
				StartDate:   proto.String("20240101"),
			},
			StopId:              proto.String("s1"),
			CurrentStopSequence: proto.Uint32(1),
			Timestamp:           proto.Uint64(1000),
			Position: &gtfs.Position{
				Latitude:  proto.Float32(42.35),
				Longitude: proto.Float32(-71.06),
			},
		},
	}

	input := createFeedMessageEvent("mbta", entity)
	flow.In() <- input

	results := collectResults(t, flow.Out(), 1)

	if len(results) != 1 {
		t.Fatal("expected 1 output event")
	}
	if results[0].GetAttributes().GetVehicleId() != "trip-fallback" {
		t.Errorf("got vehicle_id %q, want %q (trip ID fallback)", results[0].GetAttributes().GetVehicleId(), "trip-fallback")
	}
}

func TestVehiclePositionEventProcessorNilFeedMessage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flow := NewVehiclePositionEventProcessor(ctx)

	input := &pb.FeedMessageEvent{
		AgencyId:    "mbta",
		FeedMessage: nil,
	}
	flow.In() <- input

	results := collectResults(t, flow.Out(), 0)

	if len(results) != 0 {
		t.Errorf("expected 0 events for nil FeedMessage, got %d", len(results))
	}
}

func TestVehiclePositionEventProcessorTimestamp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flow := NewVehiclePositionEventProcessor(ctx)

	input := createFeedMessageEvent("mbta",
		createVehicleEntity("e1", "v1", "Red", "trip1", "s1",
			gtfs.VehiclePosition_IN_TRANSIT_TO, 1704067200, 42.35, -71.06, 1, 0, "20240101"),
	)
	flow.In() <- input

	results := collectResults(t, flow.Out(), 1)

	if len(results) != 1 {
		t.Fatal("expected 1 output event")
	}
	ts := results[0].GetAttributes().GetTimestamp().AsTime()
	expected := time.Unix(1704067200, 0)
	if !ts.Equal(expected) {
		t.Errorf("got timestamp %v, want %v", ts, expected)
	}
}

func TestVehiclePositionEventProcessorContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	flow := NewVehiclePositionEventProcessor(ctx)

	// Send one valid message to verify it processes
	input := createFeedMessageEvent("mbta",
		createVehicleEntity("e1", "v1", "Red", "trip1", "s1",
			gtfs.VehiclePosition_IN_TRANSIT_TO, 1000, 42.35, -71.06, 1, 0, "20240101"),
	)
	flow.In() <- input

	results := collectResults(t, flow.Out(), 1)
	if len(results) != 1 {
		t.Fatal("expected 1 event before cancellation")
	}

	// Cancel context
	cancel()

	// Give the goroutine time to exit
	time.Sleep(50 * time.Millisecond)

	// After cancellation, the processor should have stopped
	// Sending should not block indefinitely
	select {
	case flow.In() <- input:
		// Accepted into channel buffer; that's fine
	case <-time.After(100 * time.Millisecond):
		// Timed out; processor stopped consuming
	}
}
