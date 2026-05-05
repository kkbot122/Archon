// services/api-gateway/internal/ws/redis_pubsub.go
package ws

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

// RedisManager handles pushing and listening to Redis channels
type RedisManager struct {
	client *redis.Client
	hub    *Hub
}

func NewRedisManager(redisURL string, hub *Hub) *RedisManager {
	client := redis.NewClient(&redis.Options{
		Addr: redisURL, // e.g., "localhost:6379"
	})

	return &RedisManager{
		client: client,
		hub:    hub,
	}
}

// PublishManifestUpdate is called by our GraphQL resolver right after it saves to Postgres
func (rm *RedisManager) PublishManifestUpdate(ctx context.Context, projectID string, manifestJSON string) error {
	// We publish the raw JSON to a channel named after the project
	channel := "project_updates:" + projectID
	err := rm.client.Publish(ctx, channel, manifestJSON).Err()
	if err != nil {
		log.Printf("Failed to publish to Redis channel %s: %v", channel, err)
		return err
	}
	return nil
}

// ListenForUpdates runs in the background, listening for Redis messages and passing them to the WebSocket Hub
func (rm *RedisManager) ListenForUpdates(ctx context.Context) {
	// Subscribe to all project updates using a pattern
	pubsub := rm.client.PSubscribe(ctx, "project_updates:*")
	defer pubsub.Close()

	ch := pubsub.Channel()

	log.Println("Redis Pub/Sub listener started...")

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			// When a message hits Redis, send it directly to the WebSocket Hub
			// This string contains the raw updated JSON manifest
			rm.hub.Broadcast <- []byte(msg.Payload)
		}
	}
}