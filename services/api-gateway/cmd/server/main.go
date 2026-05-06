package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/kisna/archon/internal/kafka"
	"github.com/kisna/archon/services/api-gateway/internal/db"
	"github.com/kisna/archon/services/api-gateway/internal/graphql"
	"github.com/kisna/archon/services/api-gateway/internal/grpcclient"
	"github.com/kisna/archon/services/api-gateway/internal/middleware"
	"github.com/kisna/archon/services/api-gateway/internal/telemetry"
	"github.com/kisna/archon/services/api-gateway/internal/ws"
)

// getEnv fetches an environment variable or returns a default value
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func main() {
	log.Println("🚀 Booting up API Gateway...")

	// 0. Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  No .env file found, relying on system environment variables.")
	}

	// Fetch config with safe fallbacks
	port := getEnv("PORT", "4000")
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/archon?sslmode=disable")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	kafkaBroker := getEnv("KAFKA_BROKER", "127.0.0.1:9092")
	aiBrainTarget := getEnv("AI_BRAIN_TARGET", "localhost:50051")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// NEW: Initialize OpenTelemetry
	tp, err := telemetry.InitTracer("api-gateway")
	if err != nil {
		log.Printf("⚠️ Failed to initialize telemetry: %v", err)
	} else {
		defer tp.Shutdown(ctx)
	}

	// 1. Initialize Database Connection Pool (FIXED: With Pool Configuration)
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

	// 2. Initialize Redis (Pub/Sub)
	redisManager := ws.NewRedisManager(redisAddr)
	defer redisManager.Close()
	log.Println("✅ Connected to Redis")

	// 3. Initialize WebSocket Hub
	hub := ws.NewHub()
	go hub.Run()
	go redisManager.ListenForManifestUpdates(ctx, hub)

	// 4. Initialize Kafka Producer (Event Bus)
	kafkaProducer := kafka.NewProducer(kafkaBroker, kafka.TopicBuildRequested)
	defer kafkaProducer.Close()
	log.Println("✅ Connected to Kafka Producer")

	// 5. Initialize gRPC Client to AI Brain
	aiClient, err := grpcclient.NewClient(aiBrainTarget)
	if err != nil {
		log.Fatalf("❌ Failed to connect to AI Brain: %v", err)
	}
	defer aiClient.Close()
	log.Println("✅ gRPC Client ready (AI Brain Target:", aiBrainTarget, ")")

	// 6. Wire up the GraphQL API
	resolver := &graphql.Resolver{
		DB:    repo,
		AI:    aiClient,
		Kafka: kafkaProducer,
		Redis: redisManager,
	}

	srv := handler.NewDefaultServer(graphql.NewExecutableSchema(graphql.Config{Resolvers: resolver}))

	// 7. Setup HTTP Routes
	mux := http.NewServeMux()
	mux.Handle("/", playground.Handler("GraphQL playground", "/query"))
	mux.Handle("/query", srv)
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWS(hub, w, r)
	})

	// NEW: Wrap the entire mux in our middleware stack
	protectedHandler := middleware.Chain(mux, middleware.Recover, middleware.Logger, middleware.CORS)

	// 8. Configure the Production HTTP Server
	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      protectedHandler, // CHANGED FROM mux
		ReadTimeout:  15 * time.Second,  // Max time to read request headers/body
		WriteTimeout: 60 * time.Second,  // Max time to write response (higher for LLMs/WebSockets)
		IdleTimeout:  120 * time.Second, // Max time for keep-alive connections
	}

	// 9. Graceful Shutdown Setup
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