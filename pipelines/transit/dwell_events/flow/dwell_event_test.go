package dwell_events

import (
	"context"
	"testing"
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func createStopEvent(vehicleID, stopID, routeID string, eventType pb.StopEvent_EventType, timestamp time.Time) *pb.StopEvent {
	return &pb.StopEvent{
		VehicleId:     vehicleID,
		StopId:        stopID,
		RouteId:       routeID,
		EventType:     eventType,
		StopTimestamp: timestamppb.New(timestamp),
	}
}

func TestDwellEventFlow(t *testing.T) {
	tests := []struct {
		name     string
		inputs   []*pb.StopEvent
		expected []*pb.DwellTimeEvent
	}{
		{
			name: "calculates dwell time between arrival and departure",
			inputs: []*pb.StopEvent{
				createStopEvent("v1", "s1", "Red", pb.StopEvent_ARRIVAL, time.Unix(1000, 0)),
				createStopEvent("v1", "s1", "Red", pb.StopEvent_DEPARTURE, time.Unix(1060, 0)),
			},
			expected: []*pb.DwellTimeEvent{
				{
					EventId:          "v1-s1-1000-dwell",
					VehicleId:        "v1",
					StopId:           "s1",
					RouteId:          "Red",
					DwellTimeSeconds: 60,
				},
			},
		},
		{
			name: "ignores events at different stops",
			inputs: []*pb.StopEvent{
				createStopEvent("v1", "s1", "Red", pb.StopEvent_ARRIVAL, time.Unix(1000, 0)),
				createStopEvent("v1", "s2", "Red", pb.StopEvent_ARRIVAL, time.Unix(1060, 0)),
			},
			expected: []*pb.DwellTimeEvent{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flow := NewDwellEventFlow(context.Background())
			results := make([]*pb.DwellTimeEvent, 0)
			done := make(chan bool)

			// Collect results
			go func() {
				for event := range flow.Out() {
					if dwellEvent, ok := event.(*pb.DwellTimeEvent); ok {
						results = append(results, dwellEvent)
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
			}

			for i, want := range tt.expected {
				if i >= len(results) {
					break
				}
				got := results[i]
				if got.VehicleId != want.VehicleId ||
					got.StopId != want.StopId ||
					got.RouteId != want.RouteId ||
					got.DwellTimeSeconds != want.DwellTimeSeconds {
					t.Errorf("event %d: got %+v, want %+v", i, got, want)
				}
			}
		})
	}
}
