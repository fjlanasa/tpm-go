package sinks

import (
	"context"

	"github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
	"github.com/fjlanasa/tpm-go/event_server"
)

type SSESink struct {
	server *event_server.EventServer
	in     chan any
}

func NewSSESink(ctx context.Context, cfg config.SSESinkConfig) *SSESink {
	server := event_server.NewEventServer(ctx, config.EventServerConfig(cfg))
	sink := &SSESink{server: server, in: make(chan any)}
	go func() {
		defer close(sink.in)
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-sink.in:
				if !ok {
					return
				}
				if event, ok := event.(events.Event); ok {
					sink.server.Broadcast(event)
				}
			}
		}
	}()
	return sink
}

func (hs *SSESink) In() chan<- any {
	return hs.in
}
