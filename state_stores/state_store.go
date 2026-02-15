package state_stores

import (
	"context"
	"time"

	"github.com/fjlanasa/tpm-go/config"
	"google.golang.org/protobuf/proto"
)

type StateStore interface {
	Get(key string, new func() proto.Message) (proto.Message, bool)
	Set(key string, msg proto.Message, ttl time.Duration) error
	Delete(key string)
	Close()
}

func NewStateStore(ctx context.Context, cfg config.StateStoreConfig) StateStore {
	if cfg.Type == config.RedisStateStoreType {
		return NewRedisStateStore(ctx, cfg.Redis)
	}
	return NewInMemoryStateStore(cfg.InMemory)
}
