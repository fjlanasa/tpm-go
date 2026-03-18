package sources

import (
	"context"
	"testing"
	"time"

	"github.com/fjlanasa/tpm-go/config"
	"github.com/redis/go-redis/v9"
)

func requireRedis(t *testing.T) string {
	t.Helper()
	addr := "localhost:6379"
	client := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available at %s: %v", addr, err)
	}
	_ = client.Close()
	return addr
}

func TestRedisSourceReceivesMessage(t *testing.T) {
	addr := requireRedis(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	channel := "test-redis-source-" + t.Name()

	source, err := NewRedisSource(ctx, config.RedisSourceConfig{
		Addr:    addr,
		Channel: channel,
	})
	if err != nil {
		t.Fatalf("NewRedisSource() error = %v", err)
	}

	// Give the subscriber goroutine time to register.
	time.Sleep(50 * time.Millisecond)

	publisher := redis.NewClient(&redis.Options{Addr: addr})
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
	addr := requireRedis(t)

	ctx, cancel := context.WithCancel(context.Background())

	source, err := NewRedisSource(ctx, config.RedisSourceConfig{
		Addr:    addr,
		Channel: "test-cancel-" + t.Name(),
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
