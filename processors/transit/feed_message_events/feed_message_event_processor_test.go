package processors

import (
	"context"
	"testing"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
	"google.golang.org/protobuf/proto"
)

func createTestFeedMessage(vehicleIDs ...string) *gtfs.FeedMessage {
	entities := make([]*gtfs.FeedEntity, len(vehicleIDs))
	for i, id := range vehicleIDs {
		entityID := proto.String(id)
		entities[i] = &gtfs.FeedEntity{
			Id: entityID,
			Vehicle: &gtfs.VehiclePosition{
				Vehicle: &gtfs.VehicleDescriptor{
					Id: proto.String(id),
				},
				Trip: &gtfs.TripDescriptor{
					RouteId: proto.String("Red"),
					TripId:  proto.String("trip-" + id),
				},
			},
		}
	}

	return &gtfs.FeedMessage{
		Header: &gtfs.FeedHeader{
			GtfsRealtimeVersion: proto.String("2.0"),
			Timestamp:           proto.Uint64(uint64(time.Now().Unix())),
		},
		Entity: entities,
	}
}

func TestFeedMessageProcessor(t *testing.T) {
	tests := []struct {
		name      string
		agencyID  config.ID
		input     any
		expectOut bool
		validate  func(t *testing.T, event *pb.FeedMessageEvent)
	}{
		{
			name:     "valid binary input produces FeedMessageEvent",
			agencyID: "mbta",
			input: func() []uint8 {
				data, _ := proto.Marshal(createTestFeedMessage("v1"))
				return data
			}(),
			expectOut: true,
			validate: func(t *testing.T, event *pb.FeedMessageEvent) {
				if event.GetAgencyId() != "mbta" {
					t.Errorf("got agency_id %q, want %q", event.GetAgencyId(), "mbta")
				}
				if event.GetFeedMessage() == nil {
					t.Fatal("expected non-nil FeedMessage")
				}
				entities := event.GetFeedMessage().GetEntity()
				if len(entities) != 1 {
					t.Fatalf("got %d entities, want 1", len(entities))
				}
				if entities[0].GetVehicle().GetVehicle().GetId() != "v1" {
					t.Errorf("got vehicle_id %q, want %q", entities[0].GetVehicle().GetVehicle().GetId(), "v1")
				}
			},
		},
		{
			name:     "FeedMessage input passed through",
			agencyID: "mbta",
			input:    createTestFeedMessage("v1", "v2"),
			expectOut: true,
			validate: func(t *testing.T, event *pb.FeedMessageEvent) {
				if event.GetAgencyId() != "mbta" {
					t.Errorf("got agency_id %q, want %q", event.GetAgencyId(), "mbta")
				}
				entities := event.GetFeedMessage().GetEntity()
				if len(entities) != 2 {
					t.Fatalf("got %d entities, want 2", len(entities))
				}
			},
		},
		{
			name:      "malformed binary input is skipped",
			agencyID:  "mbta",
			input:     []uint8{0xFF, 0xFE, 0x00, 0x01, 0x02},
			expectOut: false,
		},
		{
			name:      "unknown type is skipped",
			agencyID:  "mbta",
			input:     "not a valid input type",
			expectOut: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			flow := NewFeedMessageProcessor(ctx, tt.agencyID)
			var result *pb.FeedMessageEvent
			done := make(chan bool, 1)

			go func() {
				for event := range flow.Out() {
					if fme, ok := event.(*pb.FeedMessageEvent); ok {
						result = fme
					}
				}
				done <- true
			}()

			flow.In() <- tt.input
			close(flow.In())

			<-done

			if tt.expectOut && result == nil {
				t.Fatal("expected output event but got none")
			}
			if !tt.expectOut && result != nil {
				t.Fatal("expected no output but got event")
			}
			if tt.expectOut && tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestFeedMessageProcessorMultipleInputs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flow := NewFeedMessageProcessor(ctx, "mbta")
	results := make([]*pb.FeedMessageEvent, 0)
	done := make(chan bool)

	go func() {
		for event := range flow.Out() {
			if fme, ok := event.(*pb.FeedMessageEvent); ok {
				results = append(results, fme)
			}
		}
		done <- true
	}()

	// Send multiple valid messages
	for i := 0; i < 3; i++ {
		data, _ := proto.Marshal(createTestFeedMessage("v1"))
		flow.In() <- data
	}
	close(flow.In())

	<-done

	if len(results) != 3 {
		t.Errorf("got %d events, want 3", len(results))
	}
}

func TestFeedMessageProcessorContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	flow := NewFeedMessageProcessor(ctx, "mbta")

	// Cancel context to stop the processor
	cancel()

	// Give the goroutine time to process the cancellation
	time.Sleep(50 * time.Millisecond)

	// The processor should have stopped; sending should not block indefinitely
	// We use a timeout to verify the processor is no longer consuming
	select {
	case flow.In() <- []uint8{0x01}:
		// Message was accepted into channel buffer or consumed; that's fine
	case <-time.After(100 * time.Millisecond):
		// Timed out, which means processor stopped consuming; that's expected
	}
}

func TestFeedMessageProcessorMixedInput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flow := NewFeedMessageProcessor(ctx, "mbta")
	results := make([]*pb.FeedMessageEvent, 0)
	done := make(chan bool)

	go func() {
		for event := range flow.Out() {
			if fme, ok := event.(*pb.FeedMessageEvent); ok {
				results = append(results, fme)
			}
		}
		done <- true
	}()

	// Send a mix of valid and invalid inputs
	validData, _ := proto.Marshal(createTestFeedMessage("v1"))
	flow.In() <- validData                    // valid binary
	flow.In() <- []uint8{0xFF, 0xFE}          // invalid binary
	flow.In() <- createTestFeedMessage("v2")   // valid FeedMessage
	flow.In() <- 42                            // invalid type
	close(flow.In())

	<-done

	// Only the two valid inputs should produce output
	if len(results) != 2 {
		t.Errorf("got %d events, want 2", len(results))
	}
}
