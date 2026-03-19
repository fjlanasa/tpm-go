package sources

import (
	"context"
	"log/slog"

	"github.com/fjlanasa/tpm-go/config"
	"github.com/redis/go-redis/v9"
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/flow"
)

type RedisSource struct {
	ctx    context.Context
	out    chan any
	client *redis.Client
	pubsub *redis.PubSub
}

func NewRedisSource(ctx context.Context, cfg config.RedisSourceConfig) (*RedisSource, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	pubsub := client.Subscribe(ctx, cfg.Channel)
	source := &RedisSource{
		ctx:    ctx,
		out:    make(chan any),
		client: client,
		pubsub: pubsub,
	}
	go source.init()
	return source, nil
}

func (s *RedisSource) init() {
	ch := s.pubsub.Channel()
	for {
		select {
		case <-s.ctx.Done():
			if err := s.pubsub.Close(); err != nil {
				slog.Error("redis source: failed to close pubsub", "error", err)
			}
			if err := s.client.Close(); err != nil {
				slog.Error("redis source: failed to close client", "error", err)
			}
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			s.out <- []byte(msg.Payload)
		}
	}
}

func (s *RedisSource) Via(operator streams.Flow) streams.Flow {
	flow.DoStream(s, operator)
	return operator
}

func (s *RedisSource) Out() <-chan any {
	return s.out
}
