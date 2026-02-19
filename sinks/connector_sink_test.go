package sinks

import (
	"context"
	"testing"
	"time"

	"github.com/fjlanasa/tpm-go/config"
)

func TestConnectorSinkWritesToChannel(t *testing.T) {
	connectorID := config.ID("test-connector")
	ch := make(chan any, 10)
	connectors := map[config.ID]chan any{
		connectorID: ch,
	}

	sink := NewConnectorSink(context.Background(), config.ConnectorConfig{ID: connectorID}, connectors)

	// Send an event to the sink
	sink.In() <- "hello"
	sink.In() <- 42

	// Events should appear on the connector channel
	select {
	case got := <-ch:
		if got != "hello" {
			t.Errorf("got %v, want %q", got, "hello")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for first event on connector channel")
	}

	select {
	case got := <-ch:
		if got != 42 {
			t.Errorf("got %v, want %d", got, 42)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for second event on connector channel")
	}
}

func TestConnectorSinkMultipleEvents(t *testing.T) {
	connectorID := config.ID("test-connector")
	ch := make(chan any, 100)
	connectors := map[config.ID]chan any{
		connectorID: ch,
	}

	sink := NewConnectorSink(context.Background(), config.ConnectorConfig{ID: connectorID}, connectors)

	const n = 20
	for i := 0; i < n; i++ {
		sink.In() <- i
	}

	for i := 0; i < n; i++ {
		select {
		case got := <-ch:
			if got != i {
				t.Errorf("event %d: got %v, want %d", i, got, i)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timeout waiting for event %d on connector channel", i)
		}
	}
}

func TestConnectorSinkUsesCorrectConnector(t *testing.T) {
	idA := config.ID("connector-a")
	idB := config.ID("connector-b")
	chA := make(chan any, 10)
	chB := make(chan any, 10)
	connectors := map[config.ID]chan any{
		idA: chA,
		idB: chB,
	}

	// Sink wired to connector-a
	sink := NewConnectorSink(context.Background(), config.ConnectorConfig{ID: idA}, connectors)
	sink.In() <- "for-a"

	// Should appear on chA, not chB
	select {
	case got := <-chA:
		if got != "for-a" {
			t.Errorf("got %v, want %q", got, "for-a")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event on channel A")
	}

	select {
	case got := <-chB:
		t.Errorf("unexpected event on channel B: %v", got)
	case <-time.After(20 * time.Millisecond):
		// expected: nothing on chB
	}
}
