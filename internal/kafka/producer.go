// internal/kafka/producer.go
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
}

// NewProducer connects to the Kafka broker for a specific topic
func NewProducer(brokerURL string, topic string) *Producer {
	w := &kafka.Writer{
		Addr:     kafka.TCP(brokerURL),
		Topic:    topic,
		// LeastBytes ensures messages are distributed evenly across partitions
		Balancer: &kafka.Hash{}, 
		AllowAutoTopicCreation: true,
	}
	return &Producer{writer: w}
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

// PublishBuildRequest serializes the event and fires it into the message queue
func (p *Producer) PublishBuildRequest(ctx context.Context, event BuildRequestedEvent) error {
	bytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := kafka.Message{
		// Using ProjectID as the Key ensures that multiple build requests 
		// for the same project are processed in exact order by the same consumer.
		Key:   []byte(event.ProjectID), 
		Value: bytes,
	}

	err = p.writer.WriteMessages(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to write message to kafka: %w", err)
	}

	log.Printf("🚀 Published BuildRequest to Kafka for Project: %s", event.ProjectID)
	return nil
}