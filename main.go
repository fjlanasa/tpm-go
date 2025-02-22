package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"log/slog"

	"github.com/fjlanasa/tpm-go/config"
	"github.com/fjlanasa/tpm-go/event_server"
	"github.com/fjlanasa/tpm-go/pipelines"
	"github.com/fjlanasa/tpm-go/sinks"
	"github.com/reugn/go-streams/extension"
	"github.com/reugn/go-streams/flow"
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

	// Get config path from command line arguments, default to "config.yaml"
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	config, err := config.ReadConfig(configPath)
	if err != nil {
		panic(err)
	}

	graphConfig := config.Graph

	if graphConfig == nil {
		panic("graph config is nil")
	}

	var outlet *chan any
	if config.EventServer != nil {
		ch := make(chan any)
		outlet = &ch
	}
	graph, err := pipelines.NewGraph(ctx, *graphConfig, outlet)
	if err != nil {
		panic(err)
	}
	go graph.Run()
	go func() {
		if config.EventServer != nil {
			eventServer := event_server.NewEventServer(ctx, *config.EventServer)
			extension.NewChanSource(*outlet).Via(flow.NewPassThrough()).To(sinks.NewHttpSink(ctx, eventServer))
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down...")

}
