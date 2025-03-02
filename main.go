package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"log/slog"

	"github.com/fjlanasa/tpm-go/config"
	"github.com/fjlanasa/tpm-go/graphs"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

const defaultConfigPath = "./config/configs/default.yaml"

func main() {
	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logUrl := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if logUrl != "" {
		resource := resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("tpm-go"),
			semconv.ServiceVersionKey.String("v0.1.0"),
		)
		logExporter, err := otlploghttp.New(ctx,
			otlploghttp.WithEndpoint(logUrl),
			otlploghttp.WithInsecure(),
		)
		if err != nil {
			slog.Error("failed to initialize exporter", "error", err)
			os.Exit(1)
		}
		lp := log.NewLoggerProvider(
			log.WithProcessor(
				log.NewBatchProcessor(logExporter),
			),
			log.WithResource(resource),
		)
		defer func() {
			if err := lp.Shutdown(context.Background()); err != nil {
				fmt.Printf("failed to shutdown logger provider: %v\n", err)
			}
		}()
		logger := otelslog.NewLogger("tpm-go", otelslog.WithLoggerProvider(lp))
		slog.SetDefault(logger)
	}

	slog.Info("Starting TPM-GO")

	// Create channel for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Config path precedence: ENV > CLI arg > default path
	configPath := defaultConfigPath
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		configPath = envPath
	}

	config, err := config.ReadConfig(configPath)
	if err != nil {
		slog.Error("failed to read config", "error", err)
		os.Exit(1)
	}

	graphConfig := config.Graph

	if graphConfig == nil {
		slog.Error("graph config is nil")
		os.Exit(1)
	}

	graph, err := graphs.NewGraph(ctx, *graphConfig)
	if err != nil {
		slog.Error("failed to create graph", "error", err)
		os.Exit(1)
	}
	go graph.Run()

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down...")

}
