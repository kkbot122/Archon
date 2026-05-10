package consumer

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader  *kafka.Reader
	handler *Handler
}

// New initializes the Kafka reader with manual commit configurations
func New(brokers []string, groupID, topic string, handler *Handler) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		GroupID: groupID,
		Topic:   topic,
		// Require explicit commits so we guarantee at-least-once delivery
		CommitInterval: 0, 
		MaxBytes:       10e6, // 10MB payload max
	})
	return &Consumer{reader: r, handler: handler}
}

// Start begins the blocking event loop
func (c *Consumer) Start(ctx context.Context) {
	log.Info().Msg("🎧 Kafka Consumer started. Listening for build requests...")

	for {
		// FetchMessage blocks until a message is received or context is canceled
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				log.Info().Msg("Consumer context canceled, gracefully shutting down loop.")
				return
			}
			log.Error().Err(err).Msg("Error fetching kafka message")
			continue
		}

		log.Info().
			Str("topic", msg.Topic).
			Int("partition", msg.Partition).
			Int64("offset", msg.Offset).
			Msg("📥 Received build request payload")

		// Process the build with a hard maximum timeout (e.g., 10 minutes)
		// so a frozen Docker container doesn't lock up the worker forever.
		buildCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		err = c.handler.HandleMessage(buildCtx, msg.Value)
		cancel()

		if err != nil {
			log.Warn().Err(err).Msg("Build pipeline finished with an error. (Offset will still be committed to prevent infinite retry loops)")
		}

		// Explicit manual commit — the build is fully processed and result published
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			log.Error().Err(err).Msg("Failed to commit kafka offset")
		} else {
			log.Info().Int64("offset", msg.Offset).Msg("✅ Kafka offset committed safely")
		}
	}
}

// Close cleanly disconnects from the Kafka broker
func (c *Consumer) Close() error {
	return c.reader.Close()
}