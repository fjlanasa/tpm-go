package state_stores

import (
	"context"
	"time"

	"github.com/fjlanasa/tpm-go/internal/config"
	"google.golang.org/protobuf/proto"
)

type StateStore[T proto.Message] interface {
	Get(key string) (T, bool)
	Set(key string, msg T, ttl time.Duration)
	Upsert(key string, msg T) (T, T)
	Delete(key string)
}

func NewStateStore[T proto.Message](ctx context.Context, cfg config.StateStoreConfig, new func() T) StateStore[T] {
	if cfg.Type == config.RedisStateStoreType {
		return NewRedisStateStore[T](ctx, cfg.Redis, new)
	}
	return NewInMemoryStateStore[T](cfg.InMemory, new)
}
