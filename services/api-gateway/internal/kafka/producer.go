package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

// The event payload we send to the Stitcher
type BuildRequestedEvent struct {
	ProjectID   string    `json:"project_id"`
	VersionHash string    `json:"version_hash"`
	ManifestRaw string    `json:"manifest_raw"`
	RequestedAt time.Time `json:"requested_at"`
}

// MessageWriter interface lets us mock the Kafka writer during tests
type MessageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type Producer struct {
	writer MessageWriter
}

// NewProducer connects to the real Kafka broker
func NewProducer(broker string, topic string) *Producer {
	w := &kafka.Writer{
		Addr:     kafka.TCP(broker),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	return &Producer{writer: w}
}

// NewProducerWithWriter allows injecting a mock writer
func NewProducerWithWriter(w MessageWriter) *Producer {
	return &Producer{writer: w}
}

// PublishBuildRequest marshals the event and fires it into Kafka
func (p *Producer) PublishBuildRequest(ctx context.Context, event BuildRequestedEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(event.ProjectID), // Ensures logs for the same project stay on the same partition
		Value: payload,
	}

	return p.writer.WriteMessages(ctx, msg)
}

func (p *Producer) Close() error {
	return p.writer.Close()
}