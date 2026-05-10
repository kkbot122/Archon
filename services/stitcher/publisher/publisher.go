package publisher

import (
	"context"
	"encoding/json"

	"github.com/segmentio/kafka-go"
)

type Publisher struct {
	writer *kafka.Writer
}

func New(brokerAddress string) *Publisher {
	return &Publisher{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokerAddress),
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (p *Publisher) PublishSuccess(ctx context.Context, topic string, event BuildCompletedEvent) error {
	bytes, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(event.ProjectID), // Groups events for the same project into the same partition
		Value: bytes,
	})
}

func (p *Publisher) PublishFailure(ctx context.Context, topic string, event BuildFailedEvent) error {
	bytes, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(event.ProjectID),
		Value: bytes,
	})
}

func (p *Publisher) Close() error {
	return p.writer.Close()
}