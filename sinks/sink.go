package sinks

import (
	"context"

	"github.com/fjlanasa/tpm-go/config"
	"github.com/reugn/go-streams"
)

type Sink interface {
	streams.Sink
}

func NewSink(
	ctx context.Context,
	cfg config.SinkConfig,
	connectors map[config.ID]chan any,
) Sink {
	switch cfg.Type {
	case config.SinkTypeConsole:
		return NewLogSink(ctx, cfg.Console)
	case config.SinkTypeConnector:
		return NewConnectorSink(ctx, cfg.Connector, connectors)
	}
	return nil
}
