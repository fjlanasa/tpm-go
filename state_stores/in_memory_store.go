package state_stores

import (
	"time"

	"github.com/fjlanasa/tpm-go/config"
	"google.golang.org/protobuf/proto"
)

type InMemoryState struct {
	msg        proto.Message
	ttl        time.Duration
	expiration time.Time
}

type InMemoryStateStore struct {
	ttl    time.Duration
	states map[string]*InMemoryState
	new    func() proto.Message
}

func NewInMemoryStateStore(config config.InMemoryStateStoreConfig, new func() proto.Message) *InMemoryStateStore {
	if config.Expiry == 0 {
		config.Expiry = time.Hour
	}
	s := &InMemoryStateStore{
		ttl:    config.Expiry,
		states: make(map[string]*InMemoryState),
		new:    new,
	}
	go s.expire()
	return s
}

func (s *InMemoryStateStore) Get(key string) (proto.Message, bool) {
	state, ok := s.states[key]
	if !ok || state.expiration.Before(time.Now()) {
		return s.new(), false
	}
	return state.msg, true
}

func (s *InMemoryStateStore) Set(key string, msg proto.Message, ttl time.Duration) {
	if ttl == 0 {
		ttl = s.ttl
	}
	s.states[key] = &InMemoryState{
		msg:        msg,
		ttl:        ttl,
		expiration: time.Now().Add(ttl),
	}
}

func (s *InMemoryStateStore) Delete(key string) {
	delete(s.states, key)
}

func (s *InMemoryStateStore) Upsert(key string, msg proto.Message) (proto.Message, proto.Message) {
	old, ok := s.Get(key)
	if !ok {
		s.Set(key, msg, s.ttl)
		return s.new(), msg
	}
	s.Set(key, msg, s.ttl)
	return old, msg
}

func (s *InMemoryStateStore) expire() {
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
