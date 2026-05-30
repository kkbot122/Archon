package consumer

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
	"io"

	"github.com/rs/zerolog/log"
	"github.com/segmentio/kafka-go"

	"github.com/kisna/archon/services/stitcher/publisher"
)

// Consumer listens to build-request topics, processes them,
// retries on failure, and sends to a dead‑letter queue if retries are exhausted.
type Consumer struct {
	reader       *kafka.Reader
	handler      *Handler
	retryWriter  *kafka.Writer        // writes to the retry topic
	dlqPublisher *publisher.Publisher // publishes dead‑letter events
	maxRetries   int
}

// New creates a Consumer that subscribes to multiple topics (primary + retry),
// handles retries, and sends to a dead‑letter queue after maxRetries attempts.
// topics should contain the primary topic and the retry topic, e.g. ["builds", "builds.retry"].
func New(brokers []string, groupID string, topics []string, handler *Handler) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		GroupID:        groupID,
		GroupTopics:    topics, // subscribe to all given topics
		CommitInterval: 0,      // manual commits only
		MaxBytes:       10e6,   // 10 MB
		StartOffset:    kafka.FirstOffset,
	})

	retryTopic := topics[0]
if len(topics) > 1 {
    retryTopic = topics[1]
}

	// Retry writer publishes to the primary topic (the first in the list)
	retryWriter := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    retryTopic,
		Balancer: &kafka.LeastBytes{},
	}

	// DLQ publisher reuses the existing status publisher but with a dedicated topic
	dlqPub := publisher.New(strings.Join(brokers, ","), "build.requests.dlq")

	return &Consumer{
		reader:       reader,
		handler:      handler,
		retryWriter:  retryWriter,
		dlqPublisher: dlqPub,
		maxRetries:   3, // can be made configurable via environment
	}
}

// Start runs the blocking message processing loop.
// It exits only when the context is cancelled.
func (c *Consumer) Start(ctx context.Context) {
	log.Info().Msg("🎧 Kafka Consumer started. Listening for build requests...")

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
    if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
        log.Info().Msg("Consumer context canceled or reader closed, shutting down loop.")
        return
    }
    log.Error().Err(err).Msg("Error fetching Kafka message")
    continue
}

		log.Info().
			Str("topic", msg.Topic).
			Int("partition", msg.Partition).
			Int64("offset", msg.Offset).
			Msg("📥 Received build request payload")

		// Extract the retry count (0 if not present)
		retryCount := getRetryCount(msg.Headers)

		if retryCount >= c.maxRetries {
			// Max retries reached: send to dead‑letter queue and commit
			log.Warn().
				Str("project", string(msg.Key)).
				Int("retry_count", retryCount).
				Msg("Maximum retries reached – sending to dead‑letter queue")

			dlqErr := c.dlqPublisher.PublishStatus(ctx,
				string(msg.Key), "dead_letter", "",
				"max retries exceeded, original error in previous log entries")
			if dlqErr != nil {
				log.Error().Err(dlqErr).Msg("Failed to publish to dead‑letter queue")
			}

			if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
				log.Error().Err(commitErr).Msg("Failed to commit offset after DLQ")
			} else {
				log.Info().Int64("offset", msg.Offset).Msg("✅ Offset committed after DLQ")
			}
			continue
		}

		// Process the build with a timeout
		buildCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		err = c.handler.HandleMessage(buildCtx, msg.Value)
		cancel()

		if err != nil {
			// Transient failure: increment retry count and re‑queue
			log.Warn().
				Err(err).
				Int("current_retry", retryCount).
				Msg("Build failed – re‑queuing with incremented retry count")

			nextCount := retryCount + 1
			retryMsg := kafka.Message{
				Key:   msg.Key,
				Value: msg.Value,
				Headers: []kafka.Header{
					{
						Key:   "retry_count",
						Value: []byte(strconv.Itoa(nextCount)),
					},
				},
			}

			if writeErr := c.retryWriter.WriteMessages(ctx, retryMsg); writeErr != nil {
				log.Error().Err(writeErr).Msg("Failed to re‑queue message for retry")
			}

			// Commit the original offset so it isn’t re‑delivered
			if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
				log.Error().Err(commitErr).Msg("Failed to commit original offset after re‑queueing")
			} else {
				log.Info().
					Int64("offset", msg.Offset).
					Int("next_retry", nextCount).
					Msg("✅ Original offset committed, retry message queued")
			}
			continue
		}

		// Success – commit the offset
		if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
			log.Error().Err(commitErr).Msg("Failed to commit offset")
		} else {
			log.Info().Int64("offset", msg.Offset).Msg("✅ Kafka offset committed safely")
		}
	}
}

// Close gracefully shuts down the consumer and its internal producers.
func (c *Consumer) Close() error {
	err := c.reader.Close()
	if c.retryWriter != nil {
		if wErr := c.retryWriter.Close(); wErr != nil && err == nil {
			err = wErr
		}
	}
	return err
}

// getRetryCount reads the 'retry_count' header or returns 0.
func getRetryCount(headers []kafka.Header) int {
	for _, h := range headers {
		if h.Key == "retry_count" {
			if n, err := strconv.Atoi(string(h.Value)); err == nil {
				return n
			}
		}
	}
	return 0
}