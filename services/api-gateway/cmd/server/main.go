package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	internalKafka "github.com/kisna/archon/internal/kafka"
	"github.com/kisna/archon/services/api-gateway/internal/db"
	"github.com/kisna/archon/services/api-gateway/internal/graphql"
	"github.com/kisna/archon/services/api-gateway/internal/grpcclient"
	"github.com/kisna/archon/services/api-gateway/internal/kafka"
	"github.com/kisna/archon/services/api-gateway/internal/middleware"
	"github.com/kisna/archon/services/api-gateway/internal/telemetry"
	"github.com/kisna/archon/services/api-gateway/internal/ws"
)

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func main() {
	log.Println("🚀 Booting up API Gateway...")

	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  No .env file found, relying on system environment variables.")
	}

	port          := getEnv("PORT", "4000")
	dbURL         := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/archon?sslmode=disable")
	redisAddr     := getEnv("REDIS_ADDR", "localhost:6379")
	kafkaBroker   := getEnv("KAFKA_BROKER", "127.0.0.1:9092")
	aiBrainTarget := getEnv("AI_BRAIN_TARGET", "localhost:50051")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// OpenTelemetry
	tp, err := telemetry.InitTracer("api-gateway")
	if err != nil {
		log.Printf("⚠️ Failed to initialize telemetry: %v", err)
	} else {
		defer tp.Shutdown(ctx)
	}

	// Database
	dbConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("❌ Unable to parse database URL: %v\n", err)
	}
	dbConfig.MaxConns = 20
	dbConfig.MaxConnLifetime = 1 * time.Hour
	dbConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, dbConfig)
	if err != nil {
		log.Fatalf("❌ Unable to connect to database: %v\n", err)
	}
	defer pool.Close()
	repo := db.NewRepository(pool)
	log.Println("✅ Connected to PostgreSQL (Pooled)")

	// Redis
	redisManager := ws.NewRedisManager(redisAddr)
	defer redisManager.Close()
	log.Println("✅ Connected to Redis")

	// WebSocket Hub
	hub := ws.NewHub()
	go hub.Run()
	go redisManager.ListenForManifestUpdates(ctx, hub)

	// Kafka Producer
	kafkaProducer := kafka.NewProducer(kafkaBroker, internalKafka.TopicBuildRequests)
	defer kafkaProducer.Close()
	log.Println("✅ Connected to Kafka Producer")

	// Kafka Consumer
	gatewayConsumer := kafka.NewGatewayConsumer(redisManager)
	gatewayConsumer.Start(ctx, kafkaBroker, []string{internalKafka.TopicBuildStatus})
	defer gatewayConsumer.Close()
	log.Println("✅ Kafka Consumer started (build.status → Redis → WebSocket)")

	// gRPC client
	aiClient, err := grpcclient.NewClient(aiBrainTarget)
	if err != nil {
		log.Fatalf("❌ Failed to connect to AI Brain: %v", err)
	}
	defer func() {
		if err := aiClient.Close(); err != nil {
			log.Printf("error closing gRPC client: %v", err)
		}
	}()
	log.Println("✅ gRPC Client ready (AI Brain Target:", aiBrainTarget, ")")

	// JWKS auth cache — fails fast if ARCHON_JWKS_URL is unset or unreachable
	authCache, err := middleware.InitAuth()
	if err != nil {
		slog.Error("failed to initialise auth", "error", err)
		os.Exit(1)
	}
	log.Println("✅ JWKS auth cache initialised")

	// GraphQL server
	resolver := &graphql.Resolver{
		DB:    repo,
		AI:    aiClient,
		Kafka: kafkaProducer,
		Redis: redisManager,
	}
	gqlSrv := handler.NewDefaultServer(graphql.NewExecutableSchema(graphql.Config{Resolvers: resolver}))

	// Routes
	mux := http.NewServeMux()
	mux.Handle("/", playground.Handler("GraphQL playground", "/query"))
	mux.Handle("/query", middleware.Auth(authCache)(gqlSrv))
	mux.Handle("/ws", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    ws.ServeWS(hub, w, r)
}))

	// Global middleware stack (CORS, logging, panic recovery)
	httpHandler := middleware.Chain(mux, middleware.Recover, middleware.Logger, middleware.CORS)

	// HTTP server
	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      httpHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("🌐 Connect to http://localhost:%s/ for GraphQL playground", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ HTTP server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("🛑 Shutting down server gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("❌ HTTP server shutdown failed: %v", err)
	}
	log.Println("✅ Server stopped")
}