package sources

import (
	"context"
	"testing"
	"time"

	"github.com/fjlanasa/tpm-go/config"
)

func TestConnectorSourceEmitsEvents(t *testing.T) {
	connectorID := config.ID("test-connector")
	ch := make(chan any, 10)
	connectors := map[config.ID]chan any{
		connectorID: ch,
	}

	src := NewConnectorSource(context.Background(), config.ConnectorConfig{ID: connectorID}, connectors)

	// Put events on the connector channel
	ch <- "event-1"
	ch <- "event-2"

	// Events should be available on the source's Out() channel
	for _, want := range []string{"event-1", "event-2"} {
		select {
		case got := <-src.Out():
			if got != want {
				t.Errorf("got %v, want %q", got, want)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timeout waiting for event %q from source", want)
		}
	}
}

func TestConnectorSourceMultipleEvents(t *testing.T) {
	connectorID := config.ID("test-connector")
	const n = 20
	ch := make(chan any, n)
	connectors := map[config.ID]chan any{
		connectorID: ch,
	}

	src := NewConnectorSource(context.Background(), config.ConnectorConfig{ID: connectorID}, connectors)

	for i := 0; i < n; i++ {
		ch <- i
	}

	for i := 0; i < n; i++ {
		select {
		case got := <-src.Out():
			if got != i {
				t.Errorf("event %d: got %v, want %d", i, got, i)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timeout waiting for event %d", i)
		}
	}
}

func TestConnectorSourceUsesCorrectConnector(t *testing.T) {
	idA := config.ID("connector-a")
	idB := config.ID("connector-b")
	chA := make(chan any, 10)
	chB := make(chan any, 10)
	connectors := map[config.ID]chan any{
		idA: chA,
		idB: chB,
	}

	// Source wired to connector-a
	src := NewConnectorSource(context.Background(), config.ConnectorConfig{ID: idA}, connectors)

	chA <- "from-a"
	chB <- "from-b"

	select {
	case got := <-src.Out():
		if got != "from-a" {
			t.Errorf("got %v, want %q", got, "from-a")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event from source")
	}

	// Ensure the source is not pulling from channel B
	select {
	case got := <-src.Out():
		// The only item on chA was already consumed, so this should not return "from-b"
		// unless we accidentally picked up from chB
		t.Errorf("unexpected second event from source: %v", got)
	case <-time.After(20 * time.Millisecond):
		// expected: no more events from chA
	}
}

func TestConnectorSourceStopsOnChannelClose(t *testing.T) {
	connectorID := config.ID("test-connector")
	ch := make(chan any, 10)
	connectors := map[config.ID]chan any{
		connectorID: ch,
	}

	src := NewConnectorSource(context.Background(), config.ConnectorConfig{ID: connectorID}, connectors)

	ch <- "before-close"
	close(ch)

	// First event should arrive
	select {
	case got := <-src.Out():
		if got != "before-close" {
			t.Errorf("got %v, want %q", got, "before-close")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event before channel close")
	}

	// After close, Out() channel should be closed too
	select {
	case _, ok := <-src.Out():
		if ok {
			t.Error("expected source Out() to be closed after connector channel closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for source Out() to close")
	}
}
