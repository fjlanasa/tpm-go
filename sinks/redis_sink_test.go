package sinks

import (
	"context"
	"testing"
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

func TestRedisSinkPublishesEvent(t *testing.T) {
	mr := miniredis.RunT(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	channel := "test-redis-sink"

	// Subscribe before creating the sink so we don't miss the message.
	subscriber := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer subscriber.Close()
	pubsub := subscriber.Subscribe(ctx, channel)
	defer pubsub.Close()
	msgCh := pubsub.Channel()

	sink, err := NewRedisSink(ctx, config.RedisSinkConfig{
		Addr:    mr.Addr(),
		Channel: channel,
	})
	if err != nil {
		t.Fatalf("NewRedisSink() error = %v", err)
	}

	event := makeStopEvent("agency1", "v1", "Red")
	sink.In() <- event

	select {
	case msg := <-msgCh:
		var got pb.StopEvent
		if err := proto.Unmarshal([]byte(msg.Payload), &got); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if got.GetAttributes().GetAgencyId() != "agency1" {
			t.Errorf("got agency_id %q, want %q", got.GetAttributes().GetAgencyId(), "agency1")
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for message from redis sink")
	}
}

func TestRedisSinkInvalidTypeSkipped(t *testing.T) {
	mr := miniredis.RunT(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sink, err := NewRedisSink(ctx, config.RedisSinkConfig{
		Addr:    mr.Addr(),
		Channel: "test-invalid",
	})
	if err != nil {
		t.Fatalf("NewRedisSink() error = %v", err)
	}

	// Sending a non-proto value should not panic; it is logged and skipped.
	done := make(chan struct{})
	go func() {
		defer close(done)
		sink.In() <- "not-a-proto-message"
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Error("send did not complete in time")
	}
}

func TestRedisSinkContextCancellation(t *testing.T) {
	mr := miniredis.RunT(t)

	ctx, cancel := context.WithCancel(context.Background())

	sink, err := NewRedisSink(ctx, config.RedisSinkConfig{
		Addr:    mr.Addr(),
		Channel: "test-cancel",
	})
	if err != nil {
		t.Fatalf("NewRedisSink() error = %v", err)
	}

	cancel()
	time.Sleep(100 * time.Millisecond)

	// After cancellation, sending should not block indefinitely.
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case sink.In() <- makeStopEvent("a", "b", "c"):
		case <-time.After(200 * time.Millisecond):
		}
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Error("goroutine did not complete after context cancel")
	}
}
