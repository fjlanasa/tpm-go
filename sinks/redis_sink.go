package sinks

import (
	"context"
	"log/slog"

	"github.com/fjlanasa/tpm-go/config"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

type RedisSink struct {
	ctx     context.Context
	in      chan any
	client  *redis.Client
	channel string
}

func NewRedisSink(ctx context.Context, cfg config.RedisSinkConfig) (*RedisSink, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	sink := &RedisSink{
		ctx:     ctx,
		in:      make(chan any),
		client:  client,
		channel: cfg.Channel,
	}
	go sink.doSink(ctx)
	return sink, nil
}

func (s *RedisSink) doSink(ctx context.Context) {
	defer func() {
		if err := s.client.Close(); err != nil {
			slog.Error("redis sink: failed to close client", "error", err)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-s.in:
			if !ok {
				return
			}
			protoMsg, ok := msg.(proto.Message)
			if !ok {
				slog.Warn("redis sink: invalid message type", "msg", msg)
				continue
			}
			b, err := proto.Marshal(protoMsg)
			if err != nil {
				slog.Error("redis sink: failed to marshal message", "error", err)
				continue
			}
			if err := s.client.Publish(ctx, s.channel, b).Err(); err != nil {
				slog.Error("redis sink: failed to publish message", "channel", s.channel, "error", err)
			}
		}
	}
}

func (s *RedisSink) In() chan<- any {
	return s.in
}
