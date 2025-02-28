package sinks

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
	"github.com/reugn/go-streams"
)

type LogSink struct {
	streams.Sink
	logger *slog.Logger
	level  slog.Level
	in     chan any
	ctx    context.Context
}

func NewLogSink(ctx context.Context, cfg config.ConsoleSinkConfig) *LogSink {
	level := slog.LevelInfo
	if cfg.Level != "" {
		switch cfg.Level {
		case "info":
			level = slog.LevelInfo
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
	}
	sink := &LogSink{
		ctx:    ctx,
		logger: slog.Default(),
		in:     make(chan any),
		level:  level,
	}
	go sink.doSink(ctx)
	return sink
}

func (s *LogSink) doSink(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-s.in:
			if !ok {
				panic("channel closed")
			}
			event, ok := msg.(events.Event)
			if !ok {
				s.logger.Log(ctx, slog.LevelWarn, "invalid event", "event", msg)
				panic("invalid event")
			}
			var args []any
			if attrs := event.GetAttributes(); len(attrs) > 0 {
				for k, v := range attrs {
					args = append(args, k, v)
				}
			}
			s.logger.Log(ctx, s.level, fmt.Sprintf("Event: %s", strings.TrimPrefix(fmt.Sprintf("%T", event), "*events.")), args...)
		}
	}
}

func (s *LogSink) In() chan<- any {
	return s.in
}
