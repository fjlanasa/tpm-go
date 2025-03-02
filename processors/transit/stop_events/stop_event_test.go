package processors

import (
	"context"
	"testing"
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func createVehiclePosition(vehicleID, stopID, routeID string, status pb.StopStatus, timestamp int64) *pb.VehiclePositionEvent {
	return &pb.VehiclePositionEvent{
		Attributes: &pb.EventAttributes{
			VehicleId:  vehicleID,
			StopId:     stopID,
			RouteId:    routeID,
			StopStatus: status,
			Timestamp:  timestamppb.New(time.Unix(timestamp, 0)),
		},
	}
}

func TestStopEventProcessor(t *testing.T) {
	tests := []struct {
		name     string
		inputs   []*pb.VehiclePositionEvent
		expected []*pb.StopEvent
	}{
		{
			name: "generates arrival event",
			inputs: []*pb.VehiclePositionEvent{
				createVehiclePosition("v1", "s1", "Red", pb.StopStatus_IN_TRANSIT_TO, 1000),
				createVehiclePosition("v1", "s1", "Red", pb.StopStatus_STOPPED_AT, 1001),
			},
			expected: []*pb.StopEvent{
				{
					Attributes: &pb.EventAttributes{
						VehicleId: "v1",
						StopId:    "s1",
						RouteId:   "Red",
					},
					StopEventType: pb.StopEvent_ARRIVAL,
				},
			},
		},
		{
			name: "generates departure event",
			inputs: []*pb.VehiclePositionEvent{
				createVehiclePosition("v1", "s1", "Red", pb.StopStatus_STOPPED_AT, 1000),
				createVehiclePosition("v1", "s2", "Red", pb.StopStatus_IN_TRANSIT_TO, 1001),
			},
			expected: []*pb.StopEvent{
				{
					Attributes: &pb.EventAttributes{
						VehicleId: "v1",
						StopId:    "s1",
						RouteId:   "Red",
					},
					StopEventType: pb.StopEvent_DEPARTURE,
				},
			},
		},
		{
			name: "handles destination change while in transit",
			inputs: []*pb.VehiclePositionEvent{
				createVehiclePosition("v1", "s1", "Red", pb.StopStatus_IN_TRANSIT_TO, 1000),
				createVehiclePosition("v1", "s2", "Red", pb.StopStatus_IN_TRANSIT_TO, 1001),
			},
			expected: []*pb.StopEvent{
				{
					Attributes: &pb.EventAttributes{
						VehicleId: "v1",
						StopId:    "s1",
						RouteId:   "Red",
					},
					StopEventType: pb.StopEvent_ARRIVAL,
				},
				{
					Attributes: &pb.EventAttributes{
						VehicleId: "v1",
						StopId:    "s1",
						RouteId:   "Red",
					},
					StopEventType: pb.StopEvent_DEPARTURE,
				},
			},
		},
		{
			name: "ignores repeated in-transit updates to same stop",
			inputs: []*pb.VehiclePositionEvent{
				createVehiclePosition("v1", "s1", "Red", pb.StopStatus_IN_TRANSIT_TO, 1000),
				createVehiclePosition("v1", "s1", "Red", pb.StopStatus_IN_TRANSIT_TO, 1001),
			},
			expected: []*pb.StopEvent{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flow := NewStopEventProcessor(context.Background(), nil)
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
				if got.Attributes.VehicleId != want.Attributes.VehicleId ||
					got.Attributes.StopId != want.Attributes.StopId ||
					got.Attributes.RouteId != want.Attributes.RouteId ||
					got.StopEventType != want.StopEventType {
					t.Errorf("event %d: got %+v, want %+v", i, got, want)
				}
			}
		})
	}
}
