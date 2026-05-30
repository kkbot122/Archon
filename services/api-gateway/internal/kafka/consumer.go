package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/segmentio/kafka-go"
)

// RedisPublisher defines the boundary between Kafka and WebSockets
type RedisPublisher interface {
	PublishBuildUpdate(ctx context.Context, projectID string, payload string) error
}

// GatewayConsumer listens to build events and pipes them to Redis
type GatewayConsumer struct {
	redisPub RedisPublisher
	readers  []*kafka.Reader
}

// NewGatewayConsumer creates a new consumer
func NewGatewayConsumer(redisPub RedisPublisher) *GatewayConsumer {
	return &GatewayConsumer{
		redisPub: redisPub,
	}
}

// HandleMessage processes a raw Kafka message
func (c *GatewayConsumer) HandleMessage(ctx context.Context, payload []byte) error {
	var event map[string]interface{}
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("failed to parse kafka message: %w", err)
	}

	projectID, ok := event["project_id"].(string)
	if !ok || projectID == "" {
		return fmt.Errorf("invalid or missing project_id in kafka event")
	}

	// Push straight to Redis to fan-out to the UI
	return c.redisPub.PublishBuildUpdate(ctx, projectID, string(payload))
}

// Start kicks off the consumer goroutines. It respects ctx cancellation so
// the readers stop cleanly when the application shuts down.
func (c *GatewayConsumer) Start(ctx context.Context, broker string, topics []string) {
	for _, topic := range topics {
		r := kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{broker},
			Topic:   topic,
			GroupID: "api-gateway-consumer-group",
		})
		c.readers = append(c.readers, r)

		// Run each topic consumer in its own goroutine.
		go func(reader *kafka.Reader) {
			log.Printf("🎧 API Gateway listening to Kafka topic: %s", reader.Config().Topic)
			for {
				// ReadMessage blocks until a message arrives or ctx is cancelled.
				msg, err := reader.ReadMessage(ctx)
				if err != nil {
					// A cancelled context means a clean shutdown — not an error worth logging.
					if ctx.Err() != nil {
						log.Printf("🛑 Kafka consumer shutting down for topic: %s", reader.Config().Topic)
						return
					}
					log.Printf("⚠️ Kafka read error on topic %s: %v", reader.Config().Topic, err)
					continue
				}

				if err := c.HandleMessage(ctx, msg.Value); err != nil {
					log.Printf("⚠️ Failed to handle Kafka message from %s: %v", reader.Config().Topic, err)
				}
			}
		}(r)
	}
}

// Close shuts down all Kafka readers
func (c *GatewayConsumer) Close() {
	for _, r := range c.readers {
		if err := r.Close(); err != nil {
			log.Printf("⚠️ Error closing Kafka reader: %v", err)
		}
	}
}