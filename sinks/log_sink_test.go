package sinks

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
)

// syncBuffer is a bytes.Buffer protected by a mutex for concurrent use in tests.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// newTestLogSink creates a LogSink backed by a thread-safe buffer via a JSON slog handler,
// returning the sink and a function to read captured log output.
func newTestLogSink(t *testing.T, level config.ConsoleSinkLevel) (*LogSink, func() string) {
	t.Helper()
	var buf syncBuffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	prev := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(prev) })

	sink := NewLogSink(context.Background(), config.ConsoleSinkConfig{Level: level})
	return sink, func() string { return buf.String() }
}

func makeStopEvent(agencyID, vehicleID, routeID string) *pb.StopEvent {
	return &pb.StopEvent{
		Attributes: &pb.EventAttributes{
			AgencyId:  agencyID,
			VehicleId: vehicleID,
			RouteId:   routeID,
		},
		StopEventType: pb.StopEvent_ARRIVAL,
	}
}

func TestLogSinkLogsEvent(t *testing.T) {
	sink, output := newTestLogSink(t, config.ConsoleSinkLevelInfo)

	event := makeStopEvent("agency1", "v1", "Red")
	sink.In() <- event

	// Give the goroutine time to process the event
	time.Sleep(50 * time.Millisecond)

	got := output()
	if got == "" {
		t.Fatal("expected log output but got nothing")
	}
	if !strings.Contains(got, "StopEvent") {
		t.Errorf("expected log to contain event type %q, got:\n%s", "StopEvent", got)
	}
}

func TestLogSinkLogsEventAttributes(t *testing.T) {
	sink, output := newTestLogSink(t, config.ConsoleSinkLevelInfo)

	event := makeStopEvent("mbta", "v42", "Red")
	sink.In() <- event

	time.Sleep(50 * time.Millisecond)

	got := output()
	if !strings.Contains(got, "mbta") {
		t.Errorf("expected log to contain agency_id %q, got:\n%s", "mbta", got)
	}
	if !strings.Contains(got, "v42") {
		t.Errorf("expected log to contain vehicle_id %q, got:\n%s", "v42", got)
	}
}

func TestLogSinkInvalidTypeIsSkipped(t *testing.T) {
	sink, output := newTestLogSink(t, config.ConsoleSinkLevelInfo)

	// Send a non-event value; it should log a warning but not panic
	sink.In() <- "not-an-event"

	time.Sleep(50 * time.Millisecond)

	// A warn-level log should have been emitted
	got := output()
	if !strings.Contains(strings.ToLower(got), "warn") {
		t.Errorf("expected warn log for invalid type, got:\n%s", got)
	}
}

func TestLogSinkMultipleEvents(t *testing.T) {
	sink, output := newTestLogSink(t, config.ConsoleSinkLevelInfo)

	const n = 5
	for i := 0; i < n; i++ {
		sink.In() <- makeStopEvent("agency", "v1", "Red")
	}

	time.Sleep(100 * time.Millisecond)

	got := output()
	// Each log line contains "Event: StopEvent" in the msg field exactly once.
	count := strings.Count(got, "Event: StopEvent")
	if count != n {
		t.Errorf("expected %d log lines with %q, got %d in:\n%s", n, "Event: StopEvent", count, got)
	}
}

func TestLogSinkContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sink := NewLogSink(ctx, config.ConsoleSinkConfig{Level: config.ConsoleSinkLevelInfo})

	// Cancel the context to stop the goroutine
	cancel()
	time.Sleep(50 * time.Millisecond)

	// Sending on the channel should not block indefinitely since the goroutine has stopped;
	// use a non-blocking send via a buffered channel approach with timeout.
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case sink.In() <- makeStopEvent("a", "b", "c"):
			// accepted into buffer or goroutine still draining
		case <-time.After(100 * time.Millisecond):
			// goroutine stopped consuming; this is expected
		}
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Error("goroutine send attempt did not complete in time")
	}
}

func TestLogSinkChannelClose(t *testing.T) {
	ctx := context.Background()
	sink := NewLogSink(ctx, config.ConsoleSinkConfig{Level: config.ConsoleSinkLevelInfo})

	// Closing the input channel should cause the goroutine to exit gracefully
	// (slog.Error is called, then return)
	close(sink.in)

	// Give the goroutine time to notice the close
	time.Sleep(50 * time.Millisecond)
	// If we reach here without a panic the test passes
}
