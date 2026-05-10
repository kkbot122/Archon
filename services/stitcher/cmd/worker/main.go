package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kisna/archon/services/stitcher/builder"
	"github.com/kisna/archon/services/stitcher/consumer"
	"github.com/kisna/archon/services/stitcher/internal/config"
	"github.com/kisna/archon/services/stitcher/internal/telemetry"
	"github.com/kisna/archon/services/stitcher/library"
	"github.com/kisna/archon/services/stitcher/manifest"
	"github.com/kisna/archon/services/stitcher/publisher"
	"github.com/kisna/archon/services/stitcher/stitcher"
	"github.com/kisna/archon/services/stitcher/workspace"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// 1. Setup Structured Logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	log.Info().Msg("🚀 Booting up Archon Stitcher Worker...")

	cfg := config.Load()

	// 2. Setup Graceful Shutdown Context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. Initialize OpenTelemetry
	otelEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otelEndpoint == "" {
		otelEndpoint = "localhost:4317" // Default Jaeger port
	}
	tp, err := telemetry.InitTracer(ctx, otelEndpoint)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize OpenTelemetry. Running without tracing.")
	} else {
		defer tp.Shutdown(context.Background())
		log.Info().Str("endpoint", otelEndpoint).Msg("📡 OpenTelemetry Tracing initialized")
	}

	// 4. Start Prometheus Metrics Server
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Info().Msg("📊 Metrics server listening on :8081/metrics")
		if err := http.ListenAndServe(":8081", nil); err != nil {
			log.Error().Err(err).Msg("Metrics server stopped")
		}
	}()

	// 5. Start Workspace Garbage Collector
	go workspace.StartGarbageCollector(ctx, 1*time.Hour)

	// 6. Initialize Dependencies
	dockerBuilder, err := builder.New()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Docker daemon.")
	}

	registry, err := library.LoadRegistry(cfg.AtomicLibPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load atomic library index.")
	}

	// 7. Wire the Pipeline
	resolver := library.NewResolver(registry)
	engine := stitcher.NewEngine(resolver)
	validator := manifest.NewValidator()
	pub := publisher.New(cfg.KafkaBrokers)
	
	handler := consumer.NewHandler(validator, engine, dockerBuilder, pub)

	brokers := strings.Split(cfg.KafkaBrokers, ",")
	kafkaConsumer := consumer.New(brokers, cfg.ConsumerGroup, "architect.build.requested", handler)

	// 8. Start the Consumer
	go kafkaConsumer.Start(ctx)

	// 9. Block until OS Signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	log.Info().Msg("🛑 Stop signal received. Shutting down gracefully...")

	// Cancel context to stop GC and Consumer loops
	cancel()

	if err := kafkaConsumer.Close(); err != nil {
		log.Error().Err(err).Msg("Error closing Kafka consumer")
	}
	if err := pub.Close(); err != nil {
		log.Error().Err(err).Msg("Error closing Kafka publisher")
	}

	log.Info().Msg("Shutdown complete. Goodbye! 👋")
}