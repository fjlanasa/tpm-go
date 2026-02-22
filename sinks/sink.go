package sinks

import (
	"context"
	"fmt"

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
) (Sink, error) {
	switch cfg.Type {
	case config.SinkTypeConsole:
		return NewLogSink(ctx, cfg.Console), nil
	case config.SinkTypeConnector:
		return NewConnectorSink(ctx, cfg.Connector, connectors), nil
	case config.SinkTypeSSE:
		return NewSSESink(ctx, cfg.SSE), nil
	}
	return nil, fmt.Errorf("unknown sink type: %s", cfg.Type)
}
