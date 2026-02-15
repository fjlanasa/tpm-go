package event_server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
	"github.com/google/uuid"
)

type SubscriberId string

func NewSubscriberId() SubscriberId {
	return SubscriberId(uuid.New().String())
}

type Subscription struct {
	AgencyID          string
	RouteID           string
	StopId            string
	OriginStopId      string
	DestinationStopId string
}

type Subscriber struct {
	ID           SubscriberId
	Channel      chan any
	Subscription map[string]string
}

func NewSubscriber(id SubscriberId, subscription map[string]string) *Subscriber {
	return &Subscriber{
		ID:           id,
		Channel:      make(chan any),
		Subscription: subscription,
	}
}

type EventServer struct {
	clients    map[SubscriberId]*Subscriber
	clientsMux sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewEventServer(ctx context.Context, cfg config.EventServerConfig) *EventServer {
	server := &EventServer{
		clients: make(map[SubscriberId]*Subscriber),
	}
	server.ctx, server.cancel = context.WithCancel(ctx)

	// Create HTTP server with proper shutdown handling
	httpServer := &http.Server{
		Addr: fmt.Sprintf(":%s", cfg.Port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == cfg.Path {
				server.handleSSE(w, r)
			}
		}),
	}

	// Start server in goroutine
	go func() {
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	// Handle graceful shutdown when context is cancelled
	go func() {
		<-server.ctx.Done()
		if err := httpServer.Shutdown(context.Background()); err != nil {
			slog.Error("Error shutting down server", "error", err)
		}
	}()

	return server
}

func (es *EventServer) Subscribe(subscription map[string]string) *Subscriber {
	es.clientsMux.Lock()
	defer es.clientsMux.Unlock()
	client := NewSubscriber(NewSubscriberId(), subscription)
	es.clients[client.ID] = client
	return client
}

func (es *EventServer) Unsubscribe(client *Subscriber) {
	es.clientsMux.Lock()
	defer es.clientsMux.Unlock()
	delete(es.clients, client.ID)
	close(client.Channel)
}

func (es *EventServer) Broadcast(event events.Event) {
	es.clientsMux.RLock()
	defer es.clientsMux.RUnlock()
	eventMap := events.GetEventMap(event)

	for _, client := range es.clients {
		// Check subscription match
		match := true
		for k, v := range client.Subscription {
			if v != eventMap[k] {
				match = false
				break
			}
		}
		if match {
			fmt.Println("match")
			fmt.Println(eventMap)
			client.Channel <- eventMap
		}
	}
}

func (es *EventServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create client channel
	subscription := map[string]string{}

	for k, v := range r.URL.Query() {
		subscription[k] = v[0]
	}

	client := es.Subscribe(subscription)
	defer es.Unsubscribe(client)

	// Create notification channel for client disconnect
	ctx := r.Context()
	if ctx == nil {
		http.Error(w, "Invalid request context", http.StatusInternalServerError)
		return
	}
	notify := ctx.Done()

	// Create flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case <-notify:
			return
		case event := <-client.Channel:
			var data []byte
			var err error

			switch v := event.(type) {
			case map[string]any:
				// serialize to json
				data, err = json.Marshal(v)
			default:
				continue
			}

			if err != nil {
				continue
			}

			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
