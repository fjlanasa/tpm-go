package pipelines

import (
	"context"
	"testing"
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func createTravelStopEvent(vehicleID, tripID, stopID, routeID string, directionID string, sequence int32, timestamp time.Time) *pb.StopEvent {
	return &pb.StopEvent{
		Attributes: &pb.EventAttributes{
			VehicleId:    vehicleID,
			TripId:       tripID,
			StopId:       stopID,
			RouteId:      routeID,
			DirectionId:  directionID,
			StopSequence: sequence,
			Timestamp:    timestamppb.New(timestamp),
		},
		StopEventType: pb.StopEvent_ARRIVAL,
	}
}

func TestTravelTimeEventFlow(t *testing.T) {
	tests := []struct {
		name     string
		inputs   []*pb.StopEvent
		expected []*pb.TravelTimeEvent
	}{
		{
			name: "calculates travel time between consecutive stops",
			inputs: []*pb.StopEvent{
				createTravelStopEvent("v1", "t1", "s1", "Red", "0", 1, time.Unix(1000, 0)),
				createTravelStopEvent("v1", "t1", "s2", "Red", "0", 2, time.Unix(1180, 0)),
				createTravelStopEvent("v1", "t1", "s3", "Red", "0", 3, time.Unix(1360, 0)),
			},
			expected: []*pb.TravelTimeEvent{
				{
					Attributes: &pb.EventAttributes{
						VehicleId:         "v1",
						TripId:            "t1",
						OriginStopId:      "s1",
						DestinationStopId: "s2",
						RouteId:           "Red",
						DirectionId:       "0",
					},
					TravelTimeSeconds: 180,
				},
				{
					Attributes: &pb.EventAttributes{
						VehicleId:         "v1",
						TripId:            "t1",
						OriginStopId:      "s2",
						DestinationStopId: "s3",
						RouteId:           "Red",
						DirectionId:       "0",
					},
					TravelTimeSeconds: 180,
				},
			},
		},
		{
			name: "handles multiple trips independently",
			inputs: []*pb.StopEvent{
				createTravelStopEvent("v1", "t1", "s1", "Red", "0", 1, time.Unix(1000, 0)),
				createTravelStopEvent("v2", "t2", "s1", "Red", "0", 1, time.Unix(1060, 0)),
				createTravelStopEvent("v1", "t1", "s2", "Red", "0", 2, time.Unix(1180, 0)),
			},
			expected: []*pb.TravelTimeEvent{
				{
					Attributes: &pb.EventAttributes{
						VehicleId:         "v1",
						TripId:            "t1",
						OriginStopId:      "s1",
						DestinationStopId: "s2",
						RouteId:           "Red",
						DirectionId:       "0",
					},
					TravelTimeSeconds: 180,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flow := NewTravelTimeEventFlow(context.Background())
			results := make([]*pb.TravelTimeEvent, 0)
			done := make(chan bool)

			go func() {
				for event := range flow.Out() {
					if travelEvent, ok := event.(*pb.TravelTimeEvent); ok {
						results = append(results, travelEvent)
					}
				}
				done <- true
			}()

			for _, se := range tt.inputs {
				flow.In() <- se
			}
			close(flow.In())

			<-done

			if len(results) != len(tt.expected) {
				t.Errorf("got %d events, want %d", len(results), len(tt.expected))
				return
			}

			for i, want := range tt.expected {
				got := results[i]
				if got.GetAttributes().GetVehicleId() != want.GetAttributes().GetVehicleId() ||
					got.GetAttributes().GetTripId() != want.GetAttributes().GetTripId() ||
					got.GetAttributes().GetOriginStopId() != want.GetAttributes().GetOriginStopId() ||
					got.GetAttributes().GetDestinationStopId() != want.GetAttributes().GetDestinationStopId() ||
					got.GetAttributes().GetRouteId() != want.GetAttributes().GetRouteId() ||
					got.GetAttributes().GetDirectionId() != want.GetAttributes().GetDirectionId() ||
					got.TravelTimeSeconds != want.TravelTimeSeconds {
					t.Errorf("event %d:\ngot  %+v\nwant %+v", i, got, want)
				}
			}
		})
	}
}
