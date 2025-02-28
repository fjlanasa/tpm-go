package pipelines

import (
	"context"
	"fmt"
	"testing"
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func createHeadwayStopEvent(vehicleID, stopID, routeID string, directionID uint32, eventType pb.StopEvent_EventType, timestamp time.Time) *pb.StopEvent {
	return &pb.StopEvent{
		EventId:       fmt.Sprintf("%s-%s-%d", vehicleID, stopID, timestamp.Unix()),
		VehicleId:     vehicleID,
		StopId:        stopID,
		RouteId:       routeID,
		DirectionId:   directionID,
		EventType:     eventType,
		StopTimestamp: timestamppb.New(timestamp),
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
				createHeadwayStopEvent("v1", "s1", "Red", 0, pb.StopEvent_DEPARTURE, time.Unix(1000, 0)),
				createHeadwayStopEvent("v2", "s1", "Red", 0, pb.StopEvent_DEPARTURE, time.Unix(1180, 0)),
			},
			expected: []*pb.HeadwayTimeEvent{
				{
					EventId:             "v1-s1-1000-headway",
					LeadingVehicleId:    "v1",
					FollowingVehicleId:  "v2",
					StopId:              "s1",
					RouteId:             "Red",
					DirectionId:         0,
					HeadwayTrunkSeconds: 180,
				},
			},
		},
		{
			name: "handles different directions independently",
			inputs: []*pb.StopEvent{
				createHeadwayStopEvent("v1", "s1", "Red", 0, pb.StopEvent_DEPARTURE, time.Unix(1000, 0)),
				createHeadwayStopEvent("v2", "s1", "Red", 1, pb.StopEvent_DEPARTURE, time.Unix(1060, 0)), // Different direction
				createHeadwayStopEvent("v3", "s1", "Red", 0, pb.StopEvent_DEPARTURE, time.Unix(1180, 0)),
			},
			expected: []*pb.HeadwayTimeEvent{
				{
					LeadingVehicleId:    "v1",
					FollowingVehicleId:  "v3",
					StopId:              "s1",
					RouteId:             "Red",
					DirectionId:         0,
					HeadwayTrunkSeconds: 180,
				},
			},
		},
		{
			name: "ignores non-departure events",
			inputs: []*pb.StopEvent{
				createHeadwayStopEvent("v1", "s1", "Red", 0, pb.StopEvent_DEPARTURE, time.Unix(1000, 0)),
				createHeadwayStopEvent("v2", "s1", "Red", 0, pb.StopEvent_ARRIVAL, time.Unix(1060, 0)),
				createHeadwayStopEvent("v2", "s1", "Red", 0, pb.StopEvent_DEPARTURE, time.Unix(1180, 0)),
			},
			expected: []*pb.HeadwayTimeEvent{
				{
					LeadingVehicleId:    "v1",
					FollowingVehicleId:  "v2",
					StopId:              "s1",
					RouteId:             "Red",
					DirectionId:         0,
					HeadwayTrunkSeconds: 180,
				},
			},
		},
		{
			name: "handles multiple stops independently",
			inputs: []*pb.StopEvent{
				createHeadwayStopEvent("v1", "s1", "Red", 0, pb.StopEvent_DEPARTURE, time.Unix(1000, 0)),
				createHeadwayStopEvent("v1", "s2", "Red", 0, pb.StopEvent_DEPARTURE, time.Unix(1060, 0)),
				createHeadwayStopEvent("v2", "s1", "Red", 0, pb.StopEvent_DEPARTURE, time.Unix(1180, 0)),
				createHeadwayStopEvent("v2", "s2", "Red", 0, pb.StopEvent_DEPARTURE, time.Unix(1240, 0)),
			},
			expected: []*pb.HeadwayTimeEvent{
				{
					LeadingVehicleId:    "v1",
					FollowingVehicleId:  "v2",
					StopId:              "s1",
					RouteId:             "Red",
					DirectionId:         0,
					HeadwayTrunkSeconds: 180,
				},
				{
					LeadingVehicleId:    "v1",
					FollowingVehicleId:  "v2",
					StopId:              "s2",
					RouteId:             "Red",
					DirectionId:         0,
					HeadwayTrunkSeconds: 180,
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
				if got.RouteId != want.RouteId ||
					got.StopId != want.StopId ||
					got.LeadingVehicleId != want.LeadingVehicleId ||
					got.FollowingVehicleId != want.FollowingVehicleId ||
					got.DirectionId != want.DirectionId ||
					got.HeadwayTrunkSeconds != want.HeadwayTrunkSeconds {
					t.Errorf("event %d:\ngot  %+v\nwant %+v", i, got, want)
				}
			}
		})
	}
}
