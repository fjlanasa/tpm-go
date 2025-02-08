package pipelines

import (
	"context"

	"github.com/fjlanasa/tpm-go/internal/config"
	"github.com/fjlanasa/tpm-go/internal/sinks"
	"github.com/fjlanasa/tpm-go/internal/sources"
	"github.com/fjlanasa/tpm-go/internal/state_stores"
	"google.golang.org/protobuf/proto"
)

type Pipeline struct {
	ID         string
	Type       string
	Sources    []sources.Source
	StateStore state_stores.StateStore[proto.Message]
	Sinks      []sinks.Sink
}

func NewPipeline(ctx context.Context, id string, config config.GraphConfig, newI func() proto.Message, newO func() proto.Message) *Pipeline {
	// Find pipeline config from list of pipelines. match by id
	for _, pipeline := range config.Pipelines {
		if pipeline.ID == id {
			pipelineSources := []sources.Source{}
			for _, sourceId := range pipeline.Sources {
				for _, sourceConfig := range config.Sources {
					if sourceConfig.ID == sourceId {
						source, err := sources.NewSource(ctx, sourceConfig, newI)
						if err != nil {
							panic(err)
						}
						pipelineSources = append(pipelineSources, source)
					}
				}
			}
			stateStore := state_stores.NewStateStore(ctx, config.StateStores[pipeline.StateStore], newI)
			pipelineSinks := []sinks.Sink{}
			for _, sinkId := range pipeline.Sinks {
				for _, sinkConfig := range config.Sinks {
					if sinkConfig.ID == sinkId {
						sink := sinks.NewSink(ctx, sinkConfig, newO)
						pipelineSinks = append(pipelineSinks, sink)
					}
				}
			}

			return &Pipeline{
				ID:         pipeline.ID,
				Type:       string(pipeline.Type),
				Sources:    pipelineSources,
				StateStore: stateStore,
				Sinks:      pipelineSinks,
			}
		}
	}
	return nil
}
