package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"fmt"

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

// Pipeline implements consumer.Orchestrator to glue the build steps together
type Pipeline struct {
	engine  *stitcher.Engine
	builder *builder.DockerBuilder
	pub     *publisher.Publisher
}

func (p *Pipeline) ProcessBuild(ctx context.Context, projectID, versionHash, manifestRaw string) error {
	log.Info().Str("project_id", projectID).Msg("Starting build pipeline...")

	// 1. Parse & Validate
	m, err := manifest.ParseManifest(manifestRaw)
	if err != nil {
		_ = p.pub.PublishStatus(ctx, projectID, "BUILD_FAILED", "", "Invalid manifest JSON")
		return err
	}
	if err := manifest.Validate(m); err != nil {
		_ = p.pub.PublishStatus(ctx, projectID, "BUILD_FAILED", "", err.Error())
		return err
	}

	// 2. Setup Workspace
	// FIX 1: Pass a single combined build ID string
	buildID := fmt.Sprintf("%s-%s", projectID, versionHash)
	ws, err := workspace.Create(buildID)
	if err != nil {
		_ = p.pub.PublishStatus(ctx, projectID, "BUILD_FAILED", "", "Failed to create workspace")
		return err
	}
	// Always clean up the workspace when we're done
	defer ws.Cleanup()

	// 3. Stitch Templates
	// FIX 2: Pass ws.Path (string) instead of the Workspace pointer
	if err := p.engine.Stitch(m, ws.Path); err != nil {
		_ = p.pub.PublishStatus(ctx, projectID, "BUILD_FAILED", "", "Failed to stitch architecture: "+err.Error())
		return err
	}

	// 4. Build 
	// FIX 3: Use RunBuild with the correct signature. We use 'alpine' to quickly verify the stitched files exist.
	buildResult, err := p.builder.RunBuild(ctx, buildID, ws, "alpine:latest", []string{"ls", "-la", "/workspace"})
	if err != nil || !buildResult.Success {
		errMsg := "Build failed"
		if err != nil {
			errMsg += ": " + err.Error()
		}
		if buildResult != nil && !buildResult.Success {
			errMsg += " | Logs: " + buildResult.Logs
		}
		_ = p.pub.PublishStatus(ctx, projectID, "BUILD_FAILED", "", errMsg)
		return fmt.Errorf(errMsg)
	}

	// 5. Publish Success!
	_ = p.pub.PublishStatus(ctx, projectID, "BUILD_SUCCESS", "http://localhost:8080", "")
	return nil
}

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

	pub := publisher.New(cfg.KafkaBrokers, "build.status")
	
	pipeline := &Pipeline{
		engine:  engine,
		builder: dockerBuilder,
		pub:     pub,
	}

	handler := consumer.NewHandler(pipeline)

	brokers := strings.Split(cfg.KafkaBrokers, ",")
	kafkaConsumer := consumer.New(brokers, cfg.ConsumerGroup, "build.requests", handler)

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