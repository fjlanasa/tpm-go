package pipelines

import (
	"context"
	"testing"
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func createHeadwayStopEvent(vehicleID, stopID, routeID string, directionID string, eventType pb.StopEvent_EventType, timestamp time.Time) *pb.StopEvent {
	return &pb.StopEvent{
		Attributes: &pb.EventAttributes{
			VehicleId:   vehicleID,
			StopId:      stopID,
			RouteId:     routeID,
			DirectionId: directionID,
			Timestamp:   timestamppb.New(timestamp),
		},
		StopEventType: eventType,
	}
}

func TestHeadwayEventFlow(t *testing.T) {
	tests := []struct {
		name     string
		inputs   []*pb.StopEvent
		expected []*pb.HeadwayTimeEvent
	}{
		{
			name: "calculates headway between consecutive departures",
			inputs: []*pb.StopEvent{
				createHeadwayStopEvent("v1", "s1", "Red", "0", pb.StopEvent_DEPARTURE, time.Unix(1000, 0)),
				createHeadwayStopEvent("v2", "s1", "Red", "0", pb.StopEvent_DEPARTURE, time.Unix(1180, 0)),
			},
			expected: []*pb.HeadwayTimeEvent{
				{
					Attributes: &pb.EventAttributes{
						VehicleId:   "v1",
						StopId:      "s1",
						RouteId:     "Red",
						DirectionId: "0",
					},
					LeadingVehicleId:   "v1",
					FollowingVehicleId: "v2",
					HeadwaySeconds:     180,
				},
			},
		},
		{
			name: "handles different directions independently",
			inputs: []*pb.StopEvent{
				createHeadwayStopEvent("v1", "s1", "Red", "0", pb.StopEvent_DEPARTURE, time.Unix(1000, 0)),
				createHeadwayStopEvent("v2", "s1", "Red", "1", pb.StopEvent_DEPARTURE, time.Unix(1060, 0)), // Different direction
				createHeadwayStopEvent("v3", "s1", "Red", "0", pb.StopEvent_DEPARTURE, time.Unix(1180, 0)),
			},
			expected: []*pb.HeadwayTimeEvent{
				{
					Attributes: &pb.EventAttributes{
						VehicleId:   "v1",
						StopId:      "s1",
						RouteId:     "Red",
						DirectionId: "0",
					},
					LeadingVehicleId:   "v1",
					FollowingVehicleId: "v3",
					HeadwaySeconds:     180,
				},
			},
		},
		{
			name: "ignores non-departure events",
			inputs: []*pb.StopEvent{
				createHeadwayStopEvent("v1", "s1", "Red", "0", pb.StopEvent_DEPARTURE, time.Unix(1000, 0)),
				createHeadwayStopEvent("v2", "s1", "Red", "0", pb.StopEvent_ARRIVAL, time.Unix(1060, 0)),
				createHeadwayStopEvent("v2", "s1", "Red", "0", pb.StopEvent_DEPARTURE, time.Unix(1180, 0)),
			},
			expected: []*pb.HeadwayTimeEvent{
				{
					Attributes: &pb.EventAttributes{
						VehicleId:   "v1",
						StopId:      "s1",
						RouteId:     "Red",
						DirectionId: "0",
					},
					LeadingVehicleId:   "v1",
					FollowingVehicleId: "v2",
					HeadwaySeconds:     180,
				},
			},
		},
		{
			name: "handles multiple stops independently",
			inputs: []*pb.StopEvent{
				createHeadwayStopEvent("v1", "s1", "Red", "0", pb.StopEvent_DEPARTURE, time.Unix(1000, 0)),
				createHeadwayStopEvent("v1", "s2", "Red", "0", pb.StopEvent_DEPARTURE, time.Unix(1060, 0)),
				createHeadwayStopEvent("v2", "s1", "Red", "0", pb.StopEvent_DEPARTURE, time.Unix(1180, 0)),
				createHeadwayStopEvent("v2", "s2", "Red", "0", pb.StopEvent_DEPARTURE, time.Unix(1240, 0)),
			},
			expected: []*pb.HeadwayTimeEvent{
				{
					Attributes: &pb.EventAttributes{
						VehicleId:   "v1",
						StopId:      "s1",
						RouteId:     "Red",
						DirectionId: "0",
					},
					LeadingVehicleId:   "v1",
					FollowingVehicleId: "v2",
					HeadwaySeconds:     180,
				},
				{
					Attributes: &pb.EventAttributes{
						VehicleId:   "v1",
						StopId:      "s2",
						RouteId:     "Red",
						DirectionId: "0",
					},
					LeadingVehicleId:   "v1",
					FollowingVehicleId: "v2",
					HeadwaySeconds:     180,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flow := NewHeadwayEventFlow(context.Background())
			results := make([]*pb.HeadwayTimeEvent, 0)
			done := make(chan bool)

			// Collect results
			go func() {
				for event := range flow.Out() {
					if headwayEvent, ok := event.(*pb.HeadwayTimeEvent); ok {
						results = append(results, headwayEvent)
					}
				}
				done <- true
			}()

			// Send inputs
			for _, se := range tt.inputs {
				flow.In() <- se
			}
			close(flow.In())

			// Wait for processing to complete
			<-done

			// Verify results
			if len(results) != len(tt.expected) {
				t.Errorf("got %d events, want %d", len(results), len(tt.expected))
				return
			}

			for i, want := range tt.expected {
				got := results[i]
				if got.Attributes.RouteId != want.Attributes.RouteId ||
					got.Attributes.StopId != want.Attributes.StopId ||
					got.LeadingVehicleId != want.LeadingVehicleId ||
					got.FollowingVehicleId != want.FollowingVehicleId ||
					got.Attributes.DirectionId != want.Attributes.DirectionId ||
					got.HeadwaySeconds != want.HeadwaySeconds {
					t.Errorf("event %d:\ngot  %+v\nwant %+v", i, got, want)
				}
			}
		})
	}
}
