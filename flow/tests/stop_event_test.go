package tests

import (
	"testing"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/flow"
	"google.golang.org/protobuf/proto"
)

func createVehiclePosition(vehicleID, stopID, routeID string, status gtfs.VehiclePosition_VehicleStopStatus, timestamp int64) *gtfs.VehiclePosition {
	return &gtfs.VehiclePosition{
		Vehicle: &gtfs.VehicleDescriptor{
			Id: proto.String(vehicleID),
		},
		StopId: proto.String(stopID),
		Trip: &gtfs.TripDescriptor{
			RouteId: proto.String(routeID),
		},
		CurrentStatus: &status,
		Timestamp:     proto.Uint64(uint64(timestamp)),
	}
}

func TestStopEventFlow(t *testing.T) {
	tests := []struct {
		name     string
		inputs   []*gtfs.VehiclePosition
		expected []*pb.StopEvent
	}{
		{
			name: "generates arrival event",
			inputs: []*gtfs.VehiclePosition{
				createVehiclePosition("v1", "s1", "Red", gtfs.VehiclePosition_IN_TRANSIT_TO, 1000),
				createVehiclePosition("v1", "s1", "Red", gtfs.VehiclePosition_STOPPED_AT, 1001),
			},
			expected: []*pb.StopEvent{
				{
					VehicleId: "v1",
					StopId:    "s1",
					RouteId:   "Red",
					EventType: pb.StopEvent_ARRIVAL,
				},
			},
		},
		{
			name: "generates departure event",
			inputs: []*gtfs.VehiclePosition{
				createVehiclePosition("v1", "s1", "Red", gtfs.VehiclePosition_STOPPED_AT, 1000),
				createVehiclePosition("v1", "s2", "Red", gtfs.VehiclePosition_IN_TRANSIT_TO, 1001),
			},
			expected: []*pb.StopEvent{
				{
					VehicleId: "v1",
					StopId:    "s1",
					RouteId:   "Red",
					EventType: pb.StopEvent_DEPARTURE,
				},
			},
		},
		{
			name: "handles destination change while in transit",
			inputs: []*gtfs.VehiclePosition{
				createVehiclePosition("v1", "s1", "Red", gtfs.VehiclePosition_IN_TRANSIT_TO, 1000),
				createVehiclePosition("v1", "s2", "Red", gtfs.VehiclePosition_IN_TRANSIT_TO, 1001),
			},
			expected: []*pb.StopEvent{
				{
					VehicleId: "v1",
					StopId:    "s1",
					RouteId:   "Red",
					EventType: pb.StopEvent_ARRIVAL,
				},
				{
					VehicleId: "v1",
					StopId:    "s1",
					RouteId:   "Red",
					EventType: pb.StopEvent_DEPARTURE,
				},
			},
		},
		{
			name: "ignores repeated in-transit updates to same stop",
			inputs: []*gtfs.VehiclePosition{
				createVehiclePosition("v1", "s1", "Red", gtfs.VehiclePosition_IN_TRANSIT_TO, 1000),
				createVehiclePosition("v1", "s1", "Red", gtfs.VehiclePosition_IN_TRANSIT_TO, 1001),
			},
			expected: []*pb.StopEvent{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flow := flow.NewStopEventFlow()
			results := make([]*pb.StopEvent, 0)
			done := make(chan bool)

			// Collect results
			go func() {
				for event := range flow.Out() {
					if stopEvent, ok := event.(*pb.StopEvent); ok {
						results = append(results, stopEvent)
					}
				}
				done <- true
			}()

			// Send inputs
			for _, vp := range tt.inputs {
				flow.In() <- vp
			}
			close(flow.In())

			// Wait for processing to complete
			<-done

			// Verify results
			if len(results) != len(tt.expected) {
				t.Errorf("got %d events, want %d", len(results), len(tt.expected))
			}

			for i, want := range tt.expected {
				if i >= len(results) {
					break
				}
				got := results[i]
				if got.VehicleId != want.VehicleId ||
					got.StopId != want.StopId ||
					got.RouteId != want.RouteId ||
					got.EventType != want.EventType {
					t.Errorf("event %d: got %+v, want %+v", i, got, want)
				}
			}
		})
	}
}
