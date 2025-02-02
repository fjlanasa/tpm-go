package state_stores

import (
	"context"
	"time"

	"github.com/fjlanasa/tpm-go/internal/config"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

type RedisStateStore[T proto.Message] struct {
	ctx    context.Context
	ttl    time.Duration
	client *redis.Client
	new    func() T
}

func NewRedisStateStore[T proto.Message](
	ctx context.Context,
	config config.RedisStateStoreConfig,
	new func() T,
) StateStore[T] {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB,
	})
	return &RedisStateStore[T]{
		ctx:    ctx,
		ttl:    config.Expiry,
		client: client,
		new:    new,
	}
}

func (s *RedisStateStore[T]) Get(key string) (T, bool) {
	msgStr, err := s.client.Get(s.ctx, key).Result()
	if err != nil {
		return s.new(), false
	}
	msg := s.new()
	err = proto.Unmarshal([]byte(msgStr), msg)
	if err != nil {
		return s.new(), false
	}
	return msg, true
}

func (s *RedisStateStore[T]) Set(key string, msg T, ttl time.Duration) {
	if ttl == 0 {
		ttl = s.ttl
	}
	msgStr, err := proto.Marshal(msg)
	if err != nil {
		return
	}
	s.client.Set(s.ctx, key, msgStr, ttl)
}

func (s *RedisStateStore[T]) Delete(key string) {
	s.client.Del(s.ctx, key)
}

func (s *RedisStateStore[T]) Upsert(key string, msg T) (T, T) {
	oldMsg, _ := s.Get(key)
	s.Set(key, msg, s.ttl)
	return oldMsg, msg
}
