package pipelines

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
	"github.com/fjlanasa/tpm-go/sinks"
	"github.com/fjlanasa/tpm-go/sources"
	"github.com/fjlanasa/tpm-go/statestore"
	streams "github.com/reugn/go-streams"
	"github.com/reugn/go-streams/flow"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// mockSource implements sources.Source backed by a buffered channel.
type mockSource struct {
	out chan any
}

func newMockSource(capacity int) *mockSource {
	return &mockSource{out: make(chan any, capacity)}
}

func (s *mockSource) Via(operator streams.Flow) streams.Flow {
	flow.DoStream(s, operator)
	return operator
}

func (s *mockSource) Out() <-chan any {
	return s.out
}

// mockSink implements sinks.Sink, collecting received items.
type mockSink struct {
	in       chan any
	mu       sync.Mutex
	received []any
}

func newMockSink(capacity int) *mockSink {
	s := &mockSink{in: make(chan any, capacity)}
	go func() {
		for item := range s.in {
			s.mu.Lock()
			s.received = append(s.received, item)
			s.mu.Unlock()
		}
	}()
	return s
}

func (s *mockSink) In() chan<- any {
	return s.in
}

func (s *mockSink) getReceived() []any {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]any, len(s.received))
	copy(result, s.received)
	return result
}

func (s *mockSink) waitForCount(n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(s.getReceived()) >= n {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// Compile-time interface satisfaction checks.
var _ sources.Source = (*mockSource)(nil)
var _ sinks.Sink = (*mockSink)(nil)

func makePipelineFeedMessage() []byte {
	feed := &gtfs.FeedMessage{
		Header: &gtfs.FeedHeader{
			GtfsRealtimeVersion: proto.String("2.0"),
		},
		Entity: []*gtfs.FeedEntity{
			{
				Id: proto.String("e1"),
				Vehicle: &gtfs.VehiclePosition{
					Vehicle: &gtfs.VehicleDescriptor{
						Id: proto.String("v1"),
					},
					Trip: &gtfs.TripDescriptor{
						RouteId: proto.String("Red"),
					},
				},
			},
		},
	}
	data, _ := proto.Marshal(feed)
	return data
}

func TestNewPipelineSourceNotFound(t *testing.T) {
	ctx := context.Background()
	snk := newMockSink(10)
	cfg := config.PipelineConfig{
		ID:      "test",
		Type:    config.PipelineTypeFeedMessage,
		Sources: []config.ID{"missing-source"},
		Sinks:   []config.ID{"snk-1"},
	}
	_, err := NewPipeline(ctx, cfg,
		map[config.ID]sources.Source{},
		map[config.ID]sinks.Sink{"snk-1": snk},
		map[config.ID]statestore.StateStore{},
	)
	if err == nil {
		t.Fatal("expected error when source not found, got nil")
	}
}

func TestNewPipelineSinkNotFound(t *testing.T) {
	ctx := context.Background()
	src := newMockSource(10)
	cfg := config.PipelineConfig{
		ID:      "test",
		Type:    config.PipelineTypeFeedMessage,
		Sources: []config.ID{"src-1"},
		Sinks:   []config.ID{"missing-sink"},
	}
	_, err := NewPipeline(ctx, cfg,
		map[config.ID]sources.Source{"src-1": src},
		map[config.ID]sinks.Sink{},
		map[config.ID]statestore.StateStore{},
	)
	if err == nil {
		t.Fatal("expected error when sink not found, got nil")
	}
}

func TestPipelineRunWiresFeedMessage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := newMockSource(10)
	snk := newMockSink(100)

	cfg := config.PipelineConfig{
		ID:       "test",
		AgencyID: "mbta",
		Type:     config.PipelineTypeFeedMessage,
		Sources:  []config.ID{"src-1"},
		Sinks:    []config.ID{"snk-1"},
	}

	pipeline, err := NewPipeline(ctx, cfg,
		map[config.ID]sources.Source{"src-1": src},
		map[config.ID]sinks.Sink{"snk-1": snk},
		map[config.ID]statestore.StateStore{},
	)
	if err != nil {
		t.Fatalf("NewPipeline() error = %v", err)
	}

	pipeline.Run()

	src.out <- makePipelineFeedMessage()

	if !snk.waitForCount(1, 500*time.Millisecond) {
		t.Fatal("timeout waiting for event at sink")
	}
	received := snk.getReceived()
	if len(received) != 1 {
		t.Fatalf("got %d events, want 1", len(received))
	}
	event, ok := received[0].(*pb.FeedMessageEvent)
	if !ok {
		t.Fatalf("got type %T, want *pb.FeedMessageEvent", received[0])
	}
	if event.GetAgencyId() != "mbta" {
		t.Errorf("got agency_id %q, want %q", event.GetAgencyId(), "mbta")
	}
}

func TestPipelineRunFanOut(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := newMockSource(10)
	snk1 := newMockSink(100)
	snk2 := newMockSink(100)

	cfg := config.PipelineConfig{
		ID:       "test",
		AgencyID: "mbta",
		Type:     config.PipelineTypeFeedMessage,
		Sources:  []config.ID{"src-1"},
		Sinks:    []config.ID{"snk-1", "snk-2"},
	}

	pipeline, err := NewPipeline(ctx, cfg,
		map[config.ID]sources.Source{"src-1": src},
		map[config.ID]sinks.Sink{"snk-1": snk1, "snk-2": snk2},
		map[config.ID]statestore.StateStore{},
	)
	if err != nil {
		t.Fatalf("NewPipeline() error = %v", err)
	}

	pipeline.Run()

	src.out <- makePipelineFeedMessage()

	if !snk1.waitForCount(1, 500*time.Millisecond) {
		t.Fatal("timeout waiting for event at sink 1")
	}
	if !snk2.waitForCount(1, 500*time.Millisecond) {
		t.Fatal("timeout waiting for event at sink 2")
	}
	if len(snk1.getReceived()) != 1 {
		t.Errorf("sink 1: got %d events, want 1", len(snk1.getReceived()))
	}
	if len(snk2.getReceived()) != 1 {
		t.Errorf("sink 2: got %d events, want 1", len(snk2.getReceived()))
	}
}

func TestPipelineRunWithStateStore(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := newMockSource(10)
	snk := newMockSink(100)
	ss := statestore.NewInMemoryStateStore(config.InMemoryStateStoreConfig{Expiry: time.Hour})
	defer ss.Close()

	cfg := config.PipelineConfig{
		ID:         "test",
		AgencyID:   "mbta",
		Type:       config.PipelineTypeStopEvent,
		Sources:    []config.ID{"src-1"},
		Sinks:      []config.ID{"snk-1"},
		StateStore: "ss-1",
	}

	pipeline, err := NewPipeline(ctx, cfg,
		map[config.ID]sources.Source{"src-1": src},
		map[config.ID]sinks.Sink{"snk-1": snk},
		map[config.ID]statestore.StateStore{"ss-1": ss},
	)
	if err != nil {
		t.Fatalf("NewPipeline() error = %v", err)
	}

	pipeline.Run()

	now := time.Now()
	// Send two vehicle positions that trigger a stop arrival event.
	src.out <- &pb.VehiclePositionEvent{
		Attributes: &pb.EventAttributes{
			VehicleId:  "v1",
			StopId:     "s1",
			RouteId:    "Red",
			StopStatus: pb.StopStatus_IN_TRANSIT_TO,
			Timestamp:  timestamppb.New(now),
		},
	}
	src.out <- &pb.VehiclePositionEvent{
		Attributes: &pb.EventAttributes{
			VehicleId:  "v1",
			StopId:     "s1",
			RouteId:    "Red",
			StopStatus: pb.StopStatus_STOPPED_AT,
			Timestamp:  timestamppb.New(now.Add(time.Second)),
		},
	}

	if !snk.waitForCount(1, 500*time.Millisecond) {
		t.Fatal("timeout waiting for stop event at sink")
	}

	received := snk.getReceived()
	if len(received) < 1 {
		t.Fatal("expected at least 1 stop event")
	}
	event, ok := received[0].(*pb.StopEvent)
	if !ok {
		t.Fatalf("got type %T, want *pb.StopEvent", received[0])
	}
	if event.GetStopEventType() != pb.StopEvent_ARRIVAL {
		t.Errorf("got stop event type %v, want ARRIVAL", event.GetStopEventType())
	}
}

func TestPipelineRunMultipleMessages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := newMockSource(10)
	snk := newMockSink(100)

	cfg := config.PipelineConfig{
		ID:       "test",
		AgencyID: "mbta",
		Type:     config.PipelineTypeFeedMessage,
		Sources:  []config.ID{"src-1"},
		Sinks:    []config.ID{"snk-1"},
	}

	pipeline, err := NewPipeline(ctx, cfg,
		map[config.ID]sources.Source{"src-1": src},
		map[config.ID]sinks.Sink{"snk-1": snk},
		map[config.ID]statestore.StateStore{},
	)
	if err != nil {
		t.Fatalf("NewPipeline() error = %v", err)
	}

	pipeline.Run()

	const n = 5
	for i := 0; i < n; i++ {
		src.out <- makePipelineFeedMessage()
	}

	if !snk.waitForCount(n, 500*time.Millisecond) {
		t.Fatalf("timeout: got %d events, want %d", len(snk.getReceived()), n)
	}
}
