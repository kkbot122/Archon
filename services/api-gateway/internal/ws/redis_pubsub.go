// services/api-gateway/internal/ws/redis_pubsub.go
package ws

import (
	"context"
	"log"
	"strings"

	"github.com/redis/go-redis/v9"
)

// RedisManager handles pushing and listening to Redis channels
type RedisManager struct {
	client *redis.Client
}

// NewRedisManager creates a standalone Redis client
func NewRedisManager(redisURL string) *RedisManager {
	client := redis.NewClient(&redis.Options{
		Addr: redisURL, // e.g., "localhost:6379"
	})
	return &RedisManager{
		client: client,
	}
}

// Close gracefully shuts down the Redis connection
func (rm *RedisManager) Close() error {
	if rm.client != nil {
		return rm.client.Close()
	}
	return nil
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

// ListenForManifestUpdates runs in the background, listening for Redis messages
func (rm *RedisManager) ListenForManifestUpdates(ctx context.Context, hub *Hub) {
	pubsub := rm.client.PSubscribe(ctx, "project_updates:*")
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Println("🎧 Redis Pub/Sub listener started for manifest updates...")

	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 Redis Listener shutting down...")
			return
		case msg := <-ch:
			// msg.Channel looks like "project_updates:550e8400-e29b-41d4-a716-446655440000"
			// Let's split it to extract the Project ID
			parts := strings.Split(msg.Channel, ":")
			if len(parts) == 2 {
				projectID := parts[1]
				
				// Send the targeted message to the Hub!
				hub.Broadcast <- &Message{
					ProjectID: projectID,
					Payload:   []byte(msg.Payload),
				}
			}
		}
	}
}