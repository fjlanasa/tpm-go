package pipelines

import (
	"context"
	"fmt"

	"github.com/fjlanasa/tpm-go/config"
	"github.com/fjlanasa/tpm-go/processors"

	"github.com/fjlanasa/tpm-go/sinks"
	"github.com/fjlanasa/tpm-go/sources"
	"github.com/fjlanasa/tpm-go/statestore"
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/flow"
)

type Pipeline struct {
	ID           config.ID
	agencyID     config.ID
	pipelineType config.PipelineType
	sources      []sources.Source
	stateStore   statestore.StateStore
	sinks        []sinks.Sink
	processor    processors.Processor
}

func NewPipeline(
	ctx context.Context,
	cfg config.PipelineConfig,
	sourcesByID map[config.ID]sources.Source,
	sinksByID map[config.ID]sinks.Sink,
	stateStoresByID map[config.ID]statestore.StateStore,
) (*Pipeline, error) {
	pipelineSources := []sources.Source{}
	for _, sourceID := range cfg.Sources {
		source, found := sourcesByID[sourceID]
		if !found {
			return nil, fmt.Errorf("pipeline %q: source %q not found", cfg.ID, sourceID)
		}
		pipelineSources = append(pipelineSources, source)
	}
	stateStore := stateStoresByID[cfg.StateStore]
	pipelineSinks := []sinks.Sink{}
	for _, sinkID := range cfg.Sinks {
		sink, found := sinksByID[sinkID]
		if !found {
			return nil, fmt.Errorf("pipeline %q: sink %q not found", cfg.ID, sinkID)
		}
		pipelineSinks = append(pipelineSinks, sink)
	}
	processor := processors.NewProcessor(ctx, cfg.AgencyID, cfg.Type, stateStore)

	return &Pipeline{
		ID:           cfg.ID,
		agencyID:     cfg.AgencyID,
		pipelineType: cfg.Type,
		sources:      pipelineSources,
		stateStore:   stateStore,
		sinks:        pipelineSinks,
		processor:    processor,
	}, nil
}

func (p *Pipeline) Run() {
	sourceFlows := []streams.Flow{}
	for _, source := range p.sources {
		sourceFlows = append(sourceFlows, source.Via(flow.NewPassThrough()))
	}
	mergeFlow := flow.Merge(sourceFlows...).Via(p.processor)
	sinkLen := len(p.sinks)
	sinkFlows := flow.FanOut(mergeFlow, sinkLen)
	for i, sink := range p.sinks {
		go sinkFlows[i].To(sink)
	}
}
