package state_stores

import (
	"context"
	"time"

	"github.com/fjlanasa/tpm-go/config"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

type RedisStateStore struct {
	ctx    context.Context
	ttl    time.Duration
	client *redis.Client
}

func NewRedisStateStore(
	ctx context.Context,
	config config.RedisStateStoreConfig,
) StateStore {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB,
	})
	return &RedisStateStore{
		ctx:    ctx,
		ttl:    config.Expiry,
		client: client,
	}
}

func (s *RedisStateStore) Get(key string, new func() proto.Message) (proto.Message, bool) {
	msgStr, err := s.client.Get(s.ctx, key).Result()
	if err != nil {
		return new(), false
	}
	msg := new()
	err = proto.Unmarshal([]byte(msgStr), msg)
	if err != nil {
		return new(), false
	}
	return msg, true
}

func (s *RedisStateStore) Set(key string, msg proto.Message, ttl time.Duration) error {
	if ttl == 0 {
		ttl = s.ttl
	}
	msgStr, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	return s.client.Set(s.ctx, key, msgStr, ttl).Err()
}

func (s *RedisStateStore) Delete(key string) {
	s.client.Del(s.ctx, key)
}
