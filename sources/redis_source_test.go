package sources

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/fjlanasa/tpm-go/config"
	"github.com/redis/go-redis/v9"
)

func TestRedisSourceReceivesMessage(t *testing.T) {
	mr := miniredis.RunT(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	channel := "test-redis-source"

	source, err := NewRedisSource(ctx, config.RedisSourceConfig{
		Addr:    mr.Addr(),
		Channel: channel,
	})
	if err != nil {
		t.Fatalf("NewRedisSource() error = %v", err)
	}

	// Give the subscriber goroutine time to register.
	time.Sleep(50 * time.Millisecond)

	publisher := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer publisher.Close()

	want := "hello-redis-source"
	if err := publisher.Publish(ctx, channel, want).Err(); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	select {
	case msg := <-source.Out():
		got := string(msg.([]byte))
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for message from redis source")
	}
}

func TestRedisSourceContextCancellation(t *testing.T) {
	mr := miniredis.RunT(t)

	ctx, cancel := context.WithCancel(context.Background())

	source, err := NewRedisSource(ctx, config.RedisSourceConfig{
		Addr:    mr.Addr(),
		Channel: "test-cancel",
	})
	if err != nil {
		t.Fatalf("NewRedisSource() error = %v", err)
	}

	cancel()
	time.Sleep(100 * time.Millisecond)

	// No messages should arrive after cancellation.
	select {
	case msg := <-source.Out():
		t.Errorf("unexpected message after cancel: %v", msg)
	case <-time.After(200 * time.Millisecond):
		// expected
	}
}
