package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"log/slog"

	"github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/internal/config"
	"github.com/fjlanasa/tpm-go/internal/event_server"
	"github.com/fjlanasa/tpm-go/internal/sinks"
	dwell_events "github.com/fjlanasa/tpm-go/pipelines/transit/dwell_events/flow"
	headway_events "github.com/fjlanasa/tpm-go/pipelines/transit/headway_events/flow"
	stop_events "github.com/fjlanasa/tpm-go/pipelines/transit/stop_events/flow"
	travel_time_events "github.com/fjlanasa/tpm-go/pipelines/transit/travel_time_events/flow"
	vehicle_position_source "github.com/fjlanasa/tpm-go/pipelines/transit/vehicle_position_events/source"
	"github.com/reugn/go-streams/flow"
	"google.golang.org/protobuf/proto"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create channel for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create event server
	eventServer := event_server.NewEventServer(ctx, event_server.EventServerConfig{
		Port: "8080",
		Path: "/events",
	})

	// Create source and flows
	go func() {
		// Vehicle position events
		vpSource := flow.FanOut(vehicle_position_source.NewVehiclePositionsSource(ctx, config.SourceConfig{
			Type:     config.SourceTypeHTTP,
			AgencyID: "MBTA",
			HTTP: config.HTTPSourceConfig{
				URL:      "https://cdn.mbta.com/realtime/VehiclePositions.pb",
				Interval: "1s",
			},
		}), 3)
		go vpSource[0].Via(flow.NewPassThrough()).To(sinks.NewSink(ctx, config.SinkConfig{
			Type: config.SinkTypeHttp,
		}, func() proto.Message {
			return &events.VehiclePositionEvent{}
		}, eventServer))
		go vpSource[1].Via(flow.NewPassThrough()).To(sinks.NewSink(ctx, config.SinkConfig{
			Type: config.SinkTypeConsole,
			Console: config.ConsoleSinkConfig{
				Level: "info",
			},
		}, func() proto.Message {
			return &events.VehiclePositionEvent{}
		}, eventServer))

		// Stop events
		seSources := flow.FanOut(vpSource[2].Via(stop_events.NewStopEventFlow(ctx)), 5)
		go seSources[0].Via(flow.NewPassThrough()).To(sinks.NewSink(ctx, config.SinkConfig{
			Type: config.SinkTypeHttp,
		}, func() proto.Message {
			return &events.StopEvent{}
		}, eventServer))
		go seSources[1].Via(flow.NewPassThrough()).To(sinks.NewSink(ctx, config.SinkConfig{
			Type: config.SinkTypeConsole,
			Console: config.ConsoleSinkConfig{
				Level: "info",
			},
		}, func() proto.Message {
			return &events.StopEvent{}
		}, eventServer))

		// Headway events
		headwaySources := flow.FanOut(seSources[2].Via(headway_events.NewHeadwayEventFlow(ctx)), 2)
		go headwaySources[0].Via(flow.NewPassThrough()).To(sinks.NewSink(ctx, config.SinkConfig{
			Type: config.SinkTypeConsole,
			Console: config.ConsoleSinkConfig{
				Level: "info",
			},
		}, func() proto.Message {
			return &events.HeadwayTimeEvent{}
		}, eventServer))
		go headwaySources[1].Via(flow.NewPassThrough()).To(sinks.NewSink(ctx, config.SinkConfig{
			Type: config.SinkTypeConsole,
			Console: config.ConsoleSinkConfig{
				Level: "info",
			},
		}, func() proto.Message {
			return &events.HeadwayTimeEvent{}
		}, eventServer))

		// Dwell events
		dwellSources := flow.FanOut(seSources[3].Via(dwell_events.NewDwellEventFlow(ctx)), 2)
		go dwellSources[0].Via(flow.NewPassThrough()).To(sinks.NewSink(ctx, config.SinkConfig{
			Type: config.SinkTypeConsole,
			Console: config.ConsoleSinkConfig{
				Level: "info",
			},
		}, func() proto.Message {
			return &events.DwellTimeEvent{}
		}, eventServer))
		go dwellSources[1].Via(flow.NewPassThrough()).To(sinks.NewSink(ctx, config.SinkConfig{
			Type: config.SinkTypeConsole,
			Console: config.ConsoleSinkConfig{
				Level: "info",
			},
		}, func() proto.Message {
			return &events.DwellTimeEvent{}
		}, eventServer))

		// Travel time events
		travelTimeSources := flow.FanOut(seSources[4].Via(travel_time_events.NewTravelTimeEventFlow(ctx)), 2)
		go travelTimeSources[0].Via(flow.NewPassThrough()).To(sinks.NewSink(ctx, config.SinkConfig{
			Type: config.SinkTypeConsole,
			Console: config.ConsoleSinkConfig{
				Level: "info",
			},
		}, func() proto.Message {
			return &events.TravelTimeEvent{}
		}, eventServer))
		go travelTimeSources[1].Via(flow.NewPassThrough()).To(sinks.NewSink(ctx, config.SinkConfig{
			Type: config.SinkTypeConsole,
			Console: config.ConsoleSinkConfig{
				Level: "info",
			},
		}, func() proto.Message {
			return &events.TravelTimeEvent{}
		}, eventServer))
	}()

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down...")

}
