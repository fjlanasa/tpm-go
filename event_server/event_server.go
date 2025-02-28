package event_server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

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
	EventType         string
}

func NewSubscription(agencyID, routeID, stopId, originStopID, destinationStopID, eventType string) *Subscription {
	return &Subscription{
		AgencyID:          agencyID,
		RouteID:           routeID,
		StopId:            stopId,
		OriginStopId:      originStopID,
		DestinationStopId: destinationStopID,
		EventType:         eventType,
	}
}

type Subscriber struct {
	ID           SubscriberId
	Channel      chan any
	Subscription *Subscription
}

func NewSubscriber(id SubscriberId, subscription *Subscription) *Subscriber {
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

type EventWrapper struct {
	AgencyId          string `json:"agency_id"`
	RouteId           string `json:"route_id"`
	DirectionId       uint32 `json:"direction_id"`
	StopId            string `json:"stop_id"`
	OriginStopId      string `json:"origin_stop_id"`
	DestinationStopId string `json:"destination_stop_id"`
	EventType         string `json:"event_type"`
	Event             any    `json:"event"`
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

func (es *EventServer) Subscribe(subscription *Subscription) *Subscriber {
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

func (es *EventServer) Broadcast(event EventWrapper) {
	es.clientsMux.RLock()
	defer es.clientsMux.RUnlock()

	for _, client := range es.clients {
		// Check subscription match
		if (client.Subscription.OriginStopId == "" || client.Subscription.OriginStopId == event.OriginStopId) &&
			(client.Subscription.DestinationStopId == "" || client.Subscription.DestinationStopId == event.DestinationStopId) &&
			(client.Subscription.RouteID == "" || client.Subscription.RouteID == event.RouteId) &&
			(client.Subscription.AgencyID == "" || client.Subscription.AgencyID == event.AgencyId) &&
			(client.Subscription.EventType == "" || client.Subscription.EventType == event.EventType) {
			client.Channel <- event
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
	client := es.Subscribe(NewSubscription(
		r.URL.Query().Get("agency_id"),
		r.URL.Query().Get("route_id"),
		r.URL.Query().Get("stop_id"),
		r.URL.Query().Get("origin_stop_id"),
		r.URL.Query().Get("destination_stop_id"),
		r.URL.Query().Get("event_type"),
	))
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
			case EventWrapper:
				// serialize to json
				data, err = json.Marshal(v)
			default:
				continue
			}

			if err != nil {
				continue
			}

			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
