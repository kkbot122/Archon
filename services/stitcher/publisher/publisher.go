package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/segmentio/kafka-go"
)

// BuildStatusEvent matches what the API Gateway Consumer expects
type BuildStatusEvent struct {
	ProjectID string `json:"project_id"`
	Status    string `json:"status"`
	URL       string `json:"url,omitempty"`
	Error     string `json:"error,omitempty"`
}

type Publisher struct {
	writer *kafka.Writer
}

// New properly parses a comma-separated list of brokers
func New(brokersList string, topic string) *Publisher {
	// FIX: Split comma-separated brokers to support multi-broker clusters
	rawBrokers := strings.Split(brokersList, ",")
	cleanBrokers := make([]string, 0, len(rawBrokers))
	
	for _, b := range rawBrokers {
		cleanBrokers = append(cleanBrokers, strings.TrimSpace(b))
	}

	w := &kafka.Writer{
		Addr:     kafka.TCP(cleanBrokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	return &Publisher{writer: w}
}

// PublishStatus fires the build result back to the Gateway
func (p *Publisher) PublishStatus(ctx context.Context, projectID, status, url, errMsg string) error {
	event := BuildStatusEvent{
		ProjectID: projectID,
		Status:    status,
		URL:       url,
		Error:     errMsg,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal status event: %w", err)
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(projectID), // Ensures status updates for the same project stay ordered
		Value: payload,
	})
}

// Close gracefully shuts down the publisher
func (p *Publisher) Close() error {
	return p.writer.Close()
}