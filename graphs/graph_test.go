package graphs

import (
	"context"
	"testing"
	"time"

	"github.com/fjlanasa/tpm-go/config"
)

// minimalValidConfig returns a GraphConfig that uses a connector as the
// pipeline source and a console sink as the pipeline sink.
func minimalValidConfig() config.GraphConfig {
	return config.GraphConfig{
		Sources: map[config.ID]config.SourceConfig{},
		Sinks: map[config.ID]config.SinkConfig{
			"log": {Type: config.SinkTypeConsole},
		},
		Connectors: map[config.ID]interface{}{
			"feed-input": nil,
		},
		StateStores: map[config.ID]config.StateStoreConfig{},
		Pipelines: map[config.ID]config.PipelineConfig{
			"feed-pipe": {
				ID:       "feed-pipe",
				AgencyID: "mbta",
				Type:     config.PipelineTypeFeedMessage,
				Sources:  []config.ID{"feed-input"},
				Sinks:    []config.ID{"log"},
			},
		},
	}
}

func TestNewGraphMinimalValidConfig(t *testing.T) {
	ctx := context.Background()
	graph, err := NewGraph(ctx, minimalValidConfig())
	if err != nil {
		t.Fatalf("NewGraph() error = %v, want nil", err)
	}
	if graph == nil {
		t.Fatal("expected non-nil graph")
	}
	defer graph.Close()
}

func TestNewGraphMissingSourceReference(t *testing.T) {
	ctx := context.Background()
	cfg := config.GraphConfig{
		Sources: map[config.ID]config.SourceConfig{},
		Sinks: map[config.ID]config.SinkConfig{
			"log": {Type: config.SinkTypeConsole},
		},
		Connectors:  map[config.ID]interface{}{},
		StateStores: map[config.ID]config.StateStoreConfig{},
		Pipelines: map[config.ID]config.PipelineConfig{
			"feed-pipe": {
				ID:      "feed-pipe",
				Type:    config.PipelineTypeFeedMessage,
				Sources: []config.ID{"non-existent-source"},
				Sinks:   []config.ID{"log"},
			},
		},
	}
	_, err := NewGraph(ctx, cfg)
	if err == nil {
		t.Fatal("expected error for missing source reference, got nil")
	}
}

func TestNewGraphMissingSinkReference(t *testing.T) {
	ctx := context.Background()
	cfg := config.GraphConfig{
		Sources:     map[config.ID]config.SourceConfig{},
		Sinks:       map[config.ID]config.SinkConfig{},
		Connectors:  map[config.ID]interface{}{"feed-input": nil},
		StateStores: map[config.ID]config.StateStoreConfig{},
		Pipelines: map[config.ID]config.PipelineConfig{
			"feed-pipe": {
				ID:      "feed-pipe",
				Type:    config.PipelineTypeFeedMessage,
				Sources: []config.ID{"feed-input"},
				Sinks:   []config.ID{"non-existent-sink"},
			},
		},
	}
	_, err := NewGraph(ctx, cfg)
	if err == nil {
		t.Fatal("expected error for missing sink reference, got nil")
	}
}

func TestNewGraphInvalidSourceType(t *testing.T) {
	ctx := context.Background()
	cfg := config.GraphConfig{
		Sources: map[config.ID]config.SourceConfig{
			"bad-source": {Type: "invalid-type"},
		},
		Sinks: map[config.ID]config.SinkConfig{
			"log": {Type: config.SinkTypeConsole},
		},
		Connectors:  map[config.ID]interface{}{},
		StateStores: map[config.ID]config.StateStoreConfig{},
		Pipelines: map[config.ID]config.PipelineConfig{
			"feed-pipe": {
				ID:      "feed-pipe",
				Type:    config.PipelineTypeFeedMessage,
				Sources: []config.ID{"bad-source"},
				Sinks:   []config.ID{"log"},
			},
		},
	}
	_, err := NewGraph(ctx, cfg)
	if err == nil {
		t.Fatal("expected error for invalid source type, got nil")
	}
}

func TestGraphRunAndClose(t *testing.T) {
	ctx := context.Background()
	cfg := config.GraphConfig{
		Sources: map[config.ID]config.SourceConfig{},
		Sinks: map[config.ID]config.SinkConfig{
			"log": {Type: config.SinkTypeConsole},
		},
		Connectors: map[config.ID]interface{}{
			"feed-input": nil,
		},
		StateStores: map[config.ID]config.StateStoreConfig{
			"store": {
				Type: config.InMemoryStateStoreType,
				InMemory: config.InMemoryStateStoreConfig{
					Expiry: time.Hour,
				},
			},
		},
		Pipelines: map[config.ID]config.PipelineConfig{
			"feed-pipe": {
				ID:         "feed-pipe",
				AgencyID:   "mbta",
				Type:       config.PipelineTypeFeedMessage,
				Sources:    []config.ID{"feed-input"},
				Sinks:      []config.ID{"log"},
				StateStore: "store",
			},
		},
	}

	graph, err := NewGraph(ctx, cfg)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}

	// Run and Close should not panic.
	graph.Run()
	graph.Close()
}

func TestNewGraphWithStateStore(t *testing.T) {
	ctx := context.Background()
	cfg := config.GraphConfig{
		Sources: map[config.ID]config.SourceConfig{},
		Sinks: map[config.ID]config.SinkConfig{
			"log": {Type: config.SinkTypeConsole},
		},
		Connectors: map[config.ID]interface{}{
			"stop-input": nil,
		},
		StateStores: map[config.ID]config.StateStoreConfig{
			"vehicle-store": {
				Type: config.InMemoryStateStoreType,
				InMemory: config.InMemoryStateStoreConfig{
					Expiry: time.Hour,
				},
			},
		},
		Pipelines: map[config.ID]config.PipelineConfig{
			"stop-pipe": {
				ID:         "stop-pipe",
				AgencyID:   "mbta",
				Type:       config.PipelineTypeStopEvent,
				Sources:    []config.ID{"stop-input"},
				Sinks:      []config.ID{"log"},
				StateStore: "vehicle-store",
			},
		},
	}

	graph, err := NewGraph(ctx, cfg)
	if err != nil {
		t.Fatalf("NewGraph() with state store error = %v", err)
	}
	if graph == nil {
		t.Fatal("expected non-nil graph")
	}
	defer graph.Close()
}

func TestNewGraphMultiplePipelines(t *testing.T) {
	ctx := context.Background()
	cfg := config.GraphConfig{
		Sources: map[config.ID]config.SourceConfig{},
		Sinks: map[config.ID]config.SinkConfig{
			"log": {Type: config.SinkTypeConsole},
		},
		Connectors: map[config.ID]interface{}{
			"feed-input":  nil,
			"vp-input":    nil,
			"stop-input":  nil,
		},
		StateStores: map[config.ID]config.StateStoreConfig{
			"store": {
				Type: config.InMemoryStateStoreType,
				InMemory: config.InMemoryStateStoreConfig{
					Expiry: time.Hour,
				},
			},
		},
		Pipelines: map[config.ID]config.PipelineConfig{
			"feed-pipe": {
				ID:       "feed-pipe",
				AgencyID: "mbta",
				Type:     config.PipelineTypeFeedMessage,
				Sources:  []config.ID{"feed-input"},
				Sinks:    []config.ID{"vp-input"},
			},
			"vp-pipe": {
				ID:      "vp-pipe",
				Type:    config.PipelineTypeVehiclePosition,
				Sources: []config.ID{"vp-input"},
				Sinks:   []config.ID{"stop-input"},
			},
			"stop-pipe": {
				ID:         "stop-pipe",
				Type:       config.PipelineTypeStopEvent,
				Sources:    []config.ID{"stop-input"},
				Sinks:      []config.ID{"log"},
				StateStore: "store",
			},
		},
	}

	graph, err := NewGraph(ctx, cfg)
	if err != nil {
		t.Fatalf("NewGraph() with multiple pipelines error = %v", err)
	}
	if graph == nil {
		t.Fatal("expected non-nil graph")
	}
	defer graph.Close()
}
