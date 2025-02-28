package sinks

import (
	"context"

	"github.com/fjlanasa/tpm-go/config"
	"github.com/reugn/go-streams"
	"google.golang.org/protobuf/proto"
)

type Sink interface {
	streams.Sink
}

func NewSink(
	ctx context.Context,
	cfg config.SinkConfig,
	newO func() proto.Message,
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
