package pipelines

import (
	"github.com/fjlanasa/tpm-go/internal/config"
	"github.com/fjlanasa/tpm-go/internal/state_stores"
	"github.com/reugn/go-streams"
	"google.golang.org/protobuf/proto"
)

type Pipeline struct {
	Type       string
	Sources    []streams.Source
	StateStore state_stores.StateStore[proto.Message]
	Sinks      []streams.Sink
}

func NewPipeline(config config.PipelineConfig) *Pipeline {
	return &Pipeline{
		Type:       string(config.Type),
		Sources:    config.Sources,
		StateStore: config.StateStore,
		Sinks:      config.Sinks,
	}
}
