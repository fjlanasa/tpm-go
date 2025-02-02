package state_stores

import (
	"time"

	"github.com/fjlanasa/tpm-go/internal/config"
	"google.golang.org/protobuf/proto"
)

type InMemoryState[K comparable, T proto.Message] struct {
	msg        T
	ttl        time.Duration
	expiration time.Time
}

type InMemoryStateStore[T proto.Message] struct {
	ttl    time.Duration
	states map[string]*InMemoryState[string, T]
	new    func() T
}

func NewInMemoryStateStore[T proto.Message](config config.InMemoryStateStoreConfig, new func() T) *InMemoryStateStore[T] {
	if config.Expiry == 0 {
		config.Expiry = time.Hour
	}
	s := &InMemoryStateStore[T]{
		ttl:    config.Expiry,
		states: make(map[string]*InMemoryState[string, T]),
		new:    new,
	}
	go s.expire()
	return s
}

func (s *InMemoryStateStore[T]) Get(key string) (T, bool) {
	state, ok := s.states[key]
	if !ok || state.expiration.Before(time.Now()) {
		return s.new(), false
	}
	return state.msg, true
}

func (s *InMemoryStateStore[T]) Set(key string, msg T, ttl time.Duration) {
	if ttl == 0 {
		ttl = s.ttl
	}
	s.states[key] = &InMemoryState[string, T]{
		msg:        msg,
		ttl:        ttl,
		expiration: time.Now().Add(ttl),
	}
}

func (s *InMemoryStateStore[T]) Delete(key string) {
	delete(s.states, key)
}

func (s *InMemoryStateStore[T]) Upsert(key string, msg T) (T, T) {
	old, ok := s.Get(key)
	if !ok {
		s.Set(key, msg, s.ttl)
		return s.new(), msg
	}
	s.Set(key, msg, s.ttl)
	return old, msg
}

func (s *InMemoryStateStore[T]) expire() {
	// try every ttl duration
	for {
		time.Sleep(s.ttl)
		for key, state := range s.states {
			if state.expiration.Before(time.Now()) {
				s.Delete(key)
			}
		}
	}
}
