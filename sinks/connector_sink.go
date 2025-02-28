package sinks

import (
	"context"

	"github.com/fjlanasa/tpm-go/config"
	streams "github.com/reugn/go-streams/extension"
)

func NewConnectorSink(ctx context.Context, cfg config.ConnectorConfig, connectors map[config.ID]chan any) Sink {
	in := connectors[cfg.ID]
	return streams.NewChanSink(in)
}
