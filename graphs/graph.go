package graphs

import (
	"context"
	"fmt"

	"github.com/fjlanasa/tpm-go/config"
	"github.com/fjlanasa/tpm-go/pipelines"
	"github.com/fjlanasa/tpm-go/sinks"
	"github.com/fjlanasa/tpm-go/sources"
	"github.com/fjlanasa/tpm-go/statestore"
)

type Graph struct {
	pipelines   []pipelines.Pipeline
	stateStores []statestore.StateStore
	outlet      *chan any
}

type GraphOption func(*Graph, *map[config.ID]chan any, *config.GraphConfig)

func NewGraph(ctx context.Context, cfg config.GraphConfig, opts ...GraphOption) (*Graph, error) {
	graph := &Graph{pipelines: []pipelines.Pipeline{}}
	connectors := make(map[config.ID]chan any)

	// Set up remaining connectors
	for connectorID := range cfg.Connectors {
		connectors[connectorID] = make(chan any)
		cfg.Sources[connectorID] = config.SourceConfig{
			Type: config.SourceTypeConnector,
			Connector: config.ConnectorConfig{
				ID: connectorID,
			},
		}
		cfg.Sinks[connectorID] = config.SinkConfig{
			Type: config.SinkTypeConnector,
			Connector: config.ConnectorConfig{
				ID: connectorID,
			},
		}
	}

	// Apply options
	for _, opt := range opts {
		opt(graph, &connectors, &cfg)
	}

	sourcesByID := make(map[config.ID]sources.Source)
	for sourceID, source := range cfg.Sources {
		source, err := sources.NewSource(ctx, source, connectors)
		if err != nil {
			return nil, err
		}
		sourcesByID[sourceID] = source
	}

	sinksByID := make(map[config.ID]sinks.Sink)

	for sinkID, sinkCfg := range cfg.Sinks {
		sink, err := sinks.NewSink(ctx, sinkCfg, connectors)
		if err != nil {
			return nil, fmt.Errorf("sink %q: %w", sinkID, err)
		}
		sinksByID[sinkID] = sink
	}

	stateStoresByID := make(map[config.ID]statestore.StateStore)
	for stateStoreID, stateStoreCfg := range cfg.StateStores {
		ss := statestore.NewStateStore(ctx, stateStoreCfg)
		stateStoresByID[stateStoreID] = ss
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
