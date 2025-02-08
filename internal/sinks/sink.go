package sinks

import (
	"context"

	"github.com/fjlanasa/tpm-go/internal/config"
	"github.com/fjlanasa/tpm-go/internal/event_server"
	"github.com/reugn/go-streams"
	"google.golang.org/protobuf/proto"
)

type Sink interface {
	streams.Sink
}

func NewSink(ctx context.Context, cfg config.SinkConfig, newO func() proto.Message, eventServer *event_server.EventServer) Sink {
	switch cfg.Type {
	case config.SinkTypeConsole:
		return NewLogSink(ctx, cfg.Console)
	case config.SinkTypeHttp:
		return NewHttpSink(ctx, eventServer)
	}
	return nil
}
