package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	"github.com/fjlanasa/tpm-go/api/v1/events"
	tpmflow "github.com/fjlanasa/tpm-go/flow"
	"github.com/fjlanasa/tpm-go/source"
	"github.com/google/uuid"
	"github.com/reugn/go-streams/extension"
	"github.com/reugn/go-streams/flow"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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
}

func NewEventServer() *EventServer {
	return &EventServer{
		clients: make(map[SubscriberId]*Subscriber),
	}
}

func (es *EventServer) addClient(subscription *Subscription) *Subscriber {
	es.clientsMux.Lock()
	defer es.clientsMux.Unlock()
	client := NewSubscriber(NewSubscriberId(), subscription)
	es.clients[client.ID] = client
	return client
}

func (es *EventServer) removeClient(client *Subscriber) {
	es.clientsMux.Lock()
	defer es.clientsMux.Unlock()
	delete(es.clients, client.ID)
	close(client.Channel)
}

type EventWrapper struct {
	AgencyId          string
	RouteId           string
	DirectionId       uint32
	StopId            string
	OriginStopId      string
	DestinationStopId string
	EventType         string
	Event             proto.Message
}

func (es *EventServer) broadcast(event EventWrapper) {
	es.clientsMux.RLock()
	defer es.clientsMux.RUnlock()

	for _, client := range es.clients {
		// Check subscription match
		if (client.Subscription.OriginStopId == "" || client.Subscription.OriginStopId == event.OriginStopId) &&
			(client.Subscription.DestinationStopId == "" || client.Subscription.DestinationStopId == event.DestinationStopId) &&
			(client.Subscription.RouteID == "" || client.Subscription.RouteID == event.RouteId) &&
			(client.Subscription.AgencyID == "" || client.Subscription.AgencyID == event.AgencyId) &&
			(client.Subscription.EventType == "" || client.Subscription.EventType == event.EventType) {
			client.Channel <- event.Event
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
	client := es.addClient(NewSubscription(
		r.URL.Query().Get("agency_id"),
		r.URL.Query().Get("route_id"),
		r.URL.Query().Get("stop_id"),
		r.URL.Query().Get("origin_stop_id"),
		r.URL.Query().Get("destination_stop_id"),
		r.URL.Query().Get("event_type"),
	))
	defer es.removeClient(client)

	// Create notification channel for client disconnect
	notify := r.Context().Done()

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
			case *gtfs.VehiclePosition:
				data, err = protojson.Marshal(v)
			case *events.StopEvent:
				data, err = protojson.Marshal(v)
			case *events.DwellTimeEvent:
				data, err = protojson.Marshal(v)
			case *events.HeadwayTimeEvent:
				data, err = protojson.Marshal(v)
			case *events.TravelTimeEvent:
				data, err = protojson.Marshal(v)
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

func main() {
	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create channel for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create event server
	eventServer := NewEventServer()

	// Create HTTP server
	server := &http.Server{
		Addr: ":8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/events":
				eventServer.handleSSE(w, r)
			default:
				http.NotFound(w, r)
			}
		}),
	}

	// Start HTTP server
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	// Create output channels
	vpOutChan := make(chan any)
	stopEventOutChan := make(chan any)
	headwayOutChan := make(chan any)
	dwellOutChan := make(chan any)
	travelTimeOutChan := make(chan any)

	// Create source and flows
	go func() {
		vpSource := flow.FanOut(source.NewVehiclePositionsSource("MBTA", "https://cdn.mbta.com/realtime/VehiclePositions.pb", 1*time.Second), 2)
		go vpSource[0].Via(flow.NewPassThrough()).To(extension.NewChanSink(vpOutChan))
		seSource := flow.FanOut(vpSource[1].Via(tpmflow.NewStopEventFlow()), 4)
		go seSource[0].Via(flow.NewPassThrough()).To(extension.NewChanSink(stopEventOutChan))
		go seSource[1].Via(tpmflow.NewHeadwayEventFlow()).To(extension.NewChanSink(headwayOutChan))
		go seSource[2].Via(tpmflow.NewDwellEventFlow()).To(extension.NewChanSink(dwellOutChan))
		go seSource[3].Via(tpmflow.NewTravelTimeEventFlow()).To(extension.NewChanSink(travelTimeOutChan))
	}()

	// Process messages and broadcast to clients
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-vpOutChan:
				if vpEvent, ok := event.(*gtfs.VehiclePosition); ok {
					fmt.Println("vpEvent", vpEvent)
					eventServer.broadcast(EventWrapper{
						AgencyId:    "",
						RouteId:     vpEvent.GetTrip().GetRouteId(),
						DirectionId: vpEvent.GetTrip().GetDirectionId(),
						StopId:      vpEvent.GetStopId(),
						EventType:   "vehicle-position",
						Event:       vpEvent,
					})
				}
			case event := <-stopEventOutChan:
				if stopEvent, ok := event.(*events.StopEvent); ok {
					eventServer.broadcast(EventWrapper{
						AgencyId:    stopEvent.AgencyId,
						RouteId:     stopEvent.RouteId,
						DirectionId: stopEvent.DirectionId,
						StopId:      stopEvent.StopId,
						EventType:   "stop",
						Event:       stopEvent,
					})
				}
			case event := <-dwellOutChan:
				if dwellEvent, ok := event.(*events.DwellTimeEvent); ok {
					eventServer.broadcast(EventWrapper{
						AgencyId:    dwellEvent.AgencyId,
						RouteId:     dwellEvent.RouteId,
						DirectionId: dwellEvent.DirectionId,
						StopId:      dwellEvent.StopId,
						EventType:   "dwell",
						Event:       dwellEvent,
					})
				}
			case event := <-headwayOutChan:
				if headwayEvent, ok := event.(*events.HeadwayTimeEvent); ok {
					eventServer.broadcast(EventWrapper{
						AgencyId:    headwayEvent.AgencyId,
						RouteId:     headwayEvent.RouteId,
						DirectionId: headwayEvent.DirectionId,
						StopId:      headwayEvent.StopId,
						EventType:   "headway",
						Event:       headwayEvent,
					})
				}
			case event := <-travelTimeOutChan:
				if travelTimeEvent, ok := event.(*events.TravelTimeEvent); ok {
					eventServer.broadcast(EventWrapper{
						AgencyId:          travelTimeEvent.AgencyId,
						RouteId:           travelTimeEvent.RouteId,
						DirectionId:       travelTimeEvent.DirectionId,
						OriginStopId:      travelTimeEvent.FromStopId,
						DestinationStopId: travelTimeEvent.ToStopId,
						EventType:         "travel-time",
						Event:             travelTimeEvent,
					})
				}
			}
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down...")

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("HTTP server shutdown error: %v\n", err)
	}
}
