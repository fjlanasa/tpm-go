package sinks

import (
	"context"

	"github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/event_server"
)

type HttpSink struct {
	server *event_server.EventServer
	in     chan any
}

func NewHttpSink(ctx context.Context, server *event_server.EventServer) *HttpSink {
	sink := &HttpSink{server: server, in: make(chan any)}
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

func (hs *HttpSink) In() chan<- any {
	return hs.in
}
