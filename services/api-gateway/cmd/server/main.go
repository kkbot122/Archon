// services/api-gateway/cmd/server/main.go
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

	"github.com/kisna/archon/internal/kafka"
	"github.com/kisna/archon/services/api-gateway/internal/db"
	"github.com/kisna/archon/services/api-gateway/internal/graphql"
	"github.com/kisna/archon/services/api-gateway/internal/grpcclient"
	"github.com/kisna/archon/services/api-gateway/internal/ws"
)

func main() {
	// 1. Setup Context with Graceful Shutdown capability
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("🚀 Booting up API Gateway...")

	// 2. Initialize PostgreSQL (State)
	// Using the credentials from our docker-compose.yml
	dbURL := "postgres://archon_user:archon_password@localhost:5433/archon_db?sslmode=disable"
	repo, err := db.NewRepository(ctx, dbURL)
	if err != nil {
		log.Fatalf("Fatal: Could not connect to Postgres: %v", err)
	}
	defer repo.Pool.Close()
	log.Println("✅ Connected to PostgreSQL")

	// 3. Initialize gRPC Client (AI Brain)
	// The Python service will run on port 50051 later
	aiClient, err := grpcclient.NewClient("localhost:50051")
	if err != nil {
		log.Fatalf("Fatal: Could not connect to gRPC server: %v", err)
	}
	defer aiClient.Close()
	log.Println("✅ Connected to AI Brain (gRPC)")

	// 4. Initialize Kafka Producer (Event Bus)
	kafkaProducer := kafka.NewProducer("localhost:9092", kafka.TopicBuildRequested)
	defer kafkaProducer.Close()
	log.Println("✅ Connected to Apache Kafka")

	// 5. Initialize WebSockets & Redis (Real-Time Layer)
	hub := ws.NewHub()
	go hub.Run() // Run the hub in the background

	redisManager := ws.NewRedisManager("localhost:6379", hub)
	go redisManager.ListenForUpdates(ctx) // Listen to Redis in the background
	log.Println("✅ Connected to Redis & WebSocket Hub running")

	// 6. Wire up the GraphQL API
	resolver := &graphql.Resolver{
		DB:    repo,
		AI:    aiClient,
		Kafka: kafkaProducer,
	}
	srv := handler.NewDefaultServer(graphql.NewExecutableSchema(graphql.Config{Resolvers: resolver}))

	// 7. Define HTTP Routes
	mux := http.NewServeMux()
	
	// The GraphQL Playground (UI for testing our API)
	mux.Handle("/", playground.Handler("GraphQL playground", "/query"))
	
	// The actual GraphQL Endpoint
	mux.Handle("/query", srv)
	
	// The WebSocket Endpoint
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWS(hub, w, r)
	})

	httpServer := &http.Server{
		Addr:    ":4000",
		Handler: mux,
	}

	// 8. Start the HTTP Server in a Goroutine
	go func() {
		log.Println("🔥 API Gateway listening on http://localhost:4000")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server crash: %v", err)
		}
	}()

	// 9. Graceful Shutdown Listener
	// Wait for an interrupt signal (e.g., Ctrl+C or Docker stop)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down API Gateway gracefully...")
	
	// Give the server 5 seconds to finish active requests before killing it
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("API Gateway stopped safely.")
}