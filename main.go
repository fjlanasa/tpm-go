package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log/slog"

	"github.com/fjlanasa/tpm-go/internal/config"
	"github.com/fjlanasa/tpm-go/internal/pipelines"
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

	go func() {
		graph, err := pipelines.NewGraph(ctx, config.GraphConfig{
			Connectors: map[config.ID]config.ConnectorConfig{
				"vp-connector": {
					ID: "vp-connector",
				},
				"se-connector": {
					ID: "se-connector",
				},
				"dwell-connector": {
					ID: "dwell-connector",
				},
				"headway-connector": {
					ID: "headway-connector",
				},
				"travel-time-connector": {
					ID: "travel-time-connector",
				},
			},
			Pipelines: []config.PipelineConfig{
				{
					ID:       "feed-message-pipeline",
					AgencyID: "MBTA",
					Type:     config.PipelineTypeFeedMessage,
					Sources: []config.SourceConfig{
						{
							Type: config.SourceTypeHTTP,
							HTTP: config.HTTPSourceConfig{
								URL:      "https://cdn.mbta.com/realtime/VehiclePositions.pb",
								Interval: "1s",
							},
						},
					},
					StateStore: config.StateStoreConfig{
						Type: config.InMemoryStateStoreType,
						InMemory: config.InMemoryStateStoreConfig{
							Expiry: time.Hour,
						},
					},
					Sinks: []config.SinkConfig{
						{
							ID:   "feed-message-sink-connector",
							Type: config.SinkTypeConnector,
							Connector: config.ConnectorConfig{
								ID: "vp-connector",
							},
						},
					},
				},
				{
					ID:   "vp-pipeline",
					Type: config.PipelineTypeVehiclePosition,
					Sources: []config.SourceConfig{
						{
							Type: config.SourceTypeConnector,
							Connector: config.ConnectorConfig{
								ID: "vp-connector",
							},
						},
					},
					StateStore: config.StateStoreConfig{
						Type: config.InMemoryStateStoreType,
						InMemory: config.InMemoryStateStoreConfig{
							Expiry: time.Hour,
						},
					},
					Sinks: []config.SinkConfig{
						{
							ID:   "vp-sink-connector",
							Type: config.SinkTypeConnector,
							Connector: config.ConnectorConfig{
								ID: "se-connector",
							},
						},
						{
							ID:   "vp-sink-console",
							Type: config.SinkTypeConsole,
							Console: config.ConsoleSinkConfig{
								Level: "info",
							},
						},
						{
							ID:   "vp-sink-http",
							Type: config.SinkTypeHttp,
						},
					},
				},
				{
					ID:   "se-pipeline",
					Type: config.PipelineTypeStopEvent,
					Sources: []config.SourceConfig{
						{
							Type: config.SourceTypeConnector,
							Connector: config.ConnectorConfig{
								ID: "se-connector",
							},
						},
					},
					StateStore: config.StateStoreConfig{
						Type: config.InMemoryStateStoreType,
						InMemory: config.InMemoryStateStoreConfig{
							Expiry: time.Hour,
						},
					},
					Sinks: []config.SinkConfig{
						{
							ID:   "se-sink-headway-connector",
							Type: config.SinkTypeConnector,
							Connector: config.ConnectorConfig{
								ID: "headway-connector",
							},
						},
						{
							ID:   "se-sink-dwell-connector",
							Type: config.SinkTypeConnector,
							Connector: config.ConnectorConfig{
								ID: "dwell-connector",
							},
						},
						{
							ID:   "se-sink-travel-time-connector",
							Type: config.SinkTypeConnector,
							Connector: config.ConnectorConfig{
								ID: "travel-time-connector",
							},
						},
						{
							ID:   "se-sink-console",
							Type: config.SinkTypeConsole,
							Console: config.ConsoleSinkConfig{
								Level: "info",
							},
						},
						{
							ID:   "se-sink-http",
							Type: config.SinkTypeHttp,
						},
					},
				},
				{
					ID:   "headway-pipeline",
					Type: config.PipelineTypeHeadwayEvent,
					Sources: []config.SourceConfig{
						{
							Type: config.SourceTypeConnector,
							Connector: config.ConnectorConfig{
								ID: "headway-connector",
							},
						},
					},
					StateStore: config.StateStoreConfig{
						Type: config.InMemoryStateStoreType,
						InMemory: config.InMemoryStateStoreConfig{
							Expiry: time.Hour,
						},
					},
					Sinks: []config.SinkConfig{
						{
							ID:   "headway-sink-console",
							Type: config.SinkTypeConsole,
							Console: config.ConsoleSinkConfig{
								Level: "info",
							},
						},
						{
							ID:   "headway-sink-http",
							Type: config.SinkTypeHttp,
						},
					},
				},
				{
					ID:   "dwell-pipeline",
					Type: config.PipelineTypeDwellEvent,
					Sources: []config.SourceConfig{
						{
							Type: config.SourceTypeConnector,
							Connector: config.ConnectorConfig{
								ID: "dwell-connector",
							},
						},
					},
					StateStore: config.StateStoreConfig{
						Type: config.InMemoryStateStoreType,
						InMemory: config.InMemoryStateStoreConfig{
							Expiry: time.Hour,
						},
					},
					Sinks: []config.SinkConfig{
						{
							ID:   "dwell-sink-console",
							Type: config.SinkTypeConsole,
							Console: config.ConsoleSinkConfig{
								Level: "info",
							},
						},
						{
							ID:   "dwell-sink-http",
							Type: config.SinkTypeHttp,
						},
					},
				},
				{
					ID:   "travel-time-pipeline",
					Type: config.PipelineTypeTravelTime,
					Sources: []config.SourceConfig{
						{
							Type: config.SourceTypeConnector,
							Connector: config.ConnectorConfig{
								ID: "travel-time-connector",
							},
						},
					},
					StateStore: config.StateStoreConfig{
						Type: config.InMemoryStateStoreType,
						InMemory: config.InMemoryStateStoreConfig{
							Expiry: time.Hour,
						},
					},
					Sinks: []config.SinkConfig{
						{
							ID:   "travel-time-sink-console",
							Type: config.SinkTypeConsole,
							Console: config.ConsoleSinkConfig{
								Level: "info",
							},
						},
						{
							ID:   "travel-time-sink-http",
							Type: config.SinkTypeHttp,
						},
					},
				},
			},
		})
		if err != nil {
			panic(err)
		}
		graph.Run()
	}()

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down...")

}
