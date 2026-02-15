package pipelines

import (
	"context"
	"log/slog"
	"os"

	"github.com/fjlanasa/tpm-go/config"
	"github.com/fjlanasa/tpm-go/processors"

	"github.com/fjlanasa/tpm-go/sinks"
	"github.com/fjlanasa/tpm-go/sources"
	"github.com/fjlanasa/tpm-go/state_stores"
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/flow"
)

type Pipeline struct {
	ID           config.ID
	agencyID     config.ID
	pipelineType config.PipelineType
	sources      []sources.Source
	stateStore   state_stores.StateStore
	sinks        []sinks.Sink
	processor    processors.Processor
}

func NewPipeline(
	ctx context.Context,
	config config.PipelineConfig,
	sourcesByID map[config.ID]sources.Source,
	sinksByID map[config.ID]sinks.Sink,
	stateStoresByID map[config.ID]state_stores.StateStore,
) *Pipeline {
	// Find pipeline config from list of pipelines. match by id
	pipelineSources := []sources.Source{}
	for _, sourceConfig := range config.Sources {
		source := sourcesByID[sourceConfig]
		pipelineSources = append(pipelineSources, source)
	}
	stateStore := stateStoresByID[config.StateStore]
	pipelineSinks := []sinks.Sink{}
	for _, sinkConfig := range config.Sinks {
		sink, found := sinksByID[sinkConfig]
		if !found {
			if !found {
				slog.Error("sink not found", "sink_id", sinkConfig)
				os.Exit(1)
			}
		}
		pipelineSinks = append(pipelineSinks, sink)
	}
	processor := processors.NewProcessor(ctx, config.AgencyID, config.Type, stateStore)

	return &Pipeline{
		ID:           config.ID,
		agencyID:     config.AgencyID,
		pipelineType: config.Type,
		sources:      pipelineSources,
		stateStore:   stateStore,
		sinks:        pipelineSinks,
		processor:    processor,
	}

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
