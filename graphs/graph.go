package graphs

import (
	"context"
	"fmt"

	"github.com/fjlanasa/tpm-go/config"
	"github.com/fjlanasa/tpm-go/pipelines"
	"github.com/fjlanasa/tpm-go/sinks"
	"github.com/fjlanasa/tpm-go/sources"
	"github.com/fjlanasa/tpm-go/state_stores"
)

type Graph struct {
	pipelines   []pipelines.Pipeline
	stateStores []state_stores.StateStore
	outlet      *chan any
}

type GraphOption func(*Graph, *map[config.ID]chan any, *config.GraphConfig)

func NewGraph(ctx context.Context, cfg config.GraphConfig, opts ...GraphOption) (*Graph, error) {
	graph := &Graph{pipelines: []pipelines.Pipeline{}}
	connectors := make(map[config.ID]chan any)

	// Set up remaining connectors
	for connectorId := range cfg.Connectors {
		connectors[connectorId] = make(chan any)
		cfg.Sources[connectorId] = config.SourceConfig{
			Type: config.SourceTypeConnector,
			Connector: config.ConnectorConfig{
				ID: connectorId,
			},
		}
		cfg.Sinks[connectorId] = config.SinkConfig{
			Type: config.SinkTypeConnector,
			Connector: config.ConnectorConfig{
				ID: connectorId,
			},
		}
	}

	// Apply options
	for _, opt := range opts {
		opt(graph, &connectors, &cfg)
	}

	sourcesByID := make(map[config.ID]sources.Source)
	for sourceId, source := range cfg.Sources {
		source, err := sources.NewSource(ctx, source, connectors)
		if err != nil {
			return nil, err
		}
		sourcesByID[sourceId] = source
	}

	sinksByID := make(map[config.ID]sinks.Sink)

	for sinkId, sink := range cfg.Sinks {
		sinksByID[sinkId] = sinks.NewSink(ctx, sink, connectors)
	}

	stateStoresByID := make(map[config.ID]state_stores.StateStore)
	for stateStoreId, stateStore := range cfg.StateStores {
		ss := state_stores.NewStateStore(ctx, stateStore)
		stateStoresByID[stateStoreId] = ss
		graph.stateStores = append(graph.stateStores, ss)
	}

	for _, pipelineConfig := range cfg.Pipelines {
		p, err := pipelines.NewPipeline(
			ctx,
			pipelineConfig,
			sourcesByID,
			sinksByID,
			stateStoresByID,
		)
		if err != nil {
			return nil, fmt.Errorf("pipeline %q: %w", pipelineConfig.ID, err)
		}
		graph.pipelines = append(graph.pipelines, *p)
	}
	return graph, nil
}

func (g *Graph) Out() *chan any {
	return g.outlet
}

func (g *Graph) Run() {
	for _, pipeline := range g.pipelines {
		go pipeline.Run()
	}
}

// Close shuts down all state stores owned by the graph.
func (g *Graph) Close() {
	for _, ss := range g.stateStores {
		ss.Close()
	}
}
