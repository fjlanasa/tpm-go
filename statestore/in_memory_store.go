package statestore

import (
	"sync"
	"time"

	"github.com/fjlanasa/tpm-go/config"
	"google.golang.org/protobuf/proto"
)

type InMemoryState struct {
	msg        proto.Message
	expiration time.Time
}

type InMemoryStateStore struct {
	ttl    time.Duration
	states map[string]*InMemoryState
	mu     sync.RWMutex
	done   chan struct{}
}

func NewInMemoryStateStore(config config.InMemoryStateStoreConfig) *InMemoryStateStore {
	if config.Expiry == 0 {
		config.Expiry = time.Hour
	}
	s := &InMemoryStateStore{
		ttl:    config.Expiry,
		states: make(map[string]*InMemoryState),
		done:   make(chan struct{}),
	}
	go s.expire()
	return s
}

func (s *InMemoryStateStore) Get(key string, new func() proto.Message) (proto.Message, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.states[key]
	if !ok || state.expiration.Before(time.Now()) {
		msg := new()
		if msg == nil {
			return nil, false
		}
		return msg, false
	}
	return state.msg, true
}

func (s *InMemoryStateStore) Set(key string, msg proto.Message, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ttl == 0 {
		ttl = s.ttl
	}
	s.states[key] = &InMemoryState{
		msg:        msg,
		expiration: time.Now().Add(ttl),
	}
	return nil
}

func (s *InMemoryStateStore) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, key)
}

func (s *InMemoryStateStore) Close() {
	close(s.done)
}

func (s *InMemoryStateStore) expire() {
	// Sweep at 1/10th of TTL for more responsive cleanup, with a floor of 10s
	sweepInterval := s.ttl / 10
	if sweepInterval < 10*time.Second {
		sweepInterval = 10 * time.Second
	}
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for key, state := range s.states {
				if state.expiration.Before(now) {
					delete(s.states, key)
				}
			}
			s.mu.Unlock()
		case <-s.done:
			return
		}
	}
}
