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
	new    func() proto.Message
}

func NewRedisStateStore(
	ctx context.Context,
	config config.RedisStateStoreConfig,
	new func() proto.Message,
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
		new:    new,
	}
}

func (s *RedisStateStore) Get(key string) (proto.Message, bool) {
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

func (s *RedisStateStore) Set(key string, msg proto.Message, ttl time.Duration) {
	if ttl == 0 {
		ttl = s.ttl
	}
	msgStr, err := proto.Marshal(msg)
	if err != nil {
		return
	}
	s.client.Set(s.ctx, key, msgStr, ttl)
}

func (s *RedisStateStore) Delete(key string) {
	s.client.Del(s.ctx, key)
}

func (s *RedisStateStore) Upsert(key string, msg proto.Message) (proto.Message, proto.Message) {
	oldMsg, _ := s.Get(key)
	s.Set(key, msg, s.ttl)
	return oldMsg, msg
}
