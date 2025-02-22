package state_stores

import (
	"context"
	"time"

	"github.com/fjlanasa/tpm-go/config"
	"google.golang.org/protobuf/proto"
)

type StateStore interface {
	Get(key string) (proto.Message, bool)
	Set(key string, msg proto.Message, ttl time.Duration)
	Upsert(key string, msg proto.Message) (proto.Message, proto.Message)
	Delete(key string)
}

func NewStateStore(ctx context.Context, cfg config.StateStoreConfig, new func() proto.Message) StateStore {
	if cfg.Type == config.RedisStateStoreType {
		return NewRedisStateStore(ctx, cfg.Redis, new)
	}
	return NewInMemoryStateStore(cfg.InMemory, new)
}
