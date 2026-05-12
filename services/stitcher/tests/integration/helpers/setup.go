package helpers

import (
	"context"
	"encoding/json"
	"testing"
	"time"
	"os/exec"

	"github.com/segmentio/kafka-go"
)

// NewProducer creates a Kafka writer for the given topic.
func NewProducer(t *testing.T, topic string) *kafka.Writer {
	t.Helper()
	w := &kafka.Writer{
		Addr:     kafka.TCP("localhost:9092"),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	t.Cleanup(func() {
		w.Close()
	})
	return w
}

// NewConsumer creates a Kafka reader that only receives messages published after it starts.
func NewConsumer(t *testing.T, topic, groupID string) *kafka.Reader {
	t.Helper()
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{"localhost:9092"},
		Topic:       topic,
		GroupID:     groupID,
		StartOffset: kafka.LastOffset,
		MaxWait:     time.Second,
	})
	t.Cleanup(func() {
		r.Close()
	})
	return r
}

// PublishJSON marshals v to JSON and writes it to the given writer.
func PublishJSON(ctx context.Context, w *kafka.Writer, key string, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return w.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: b,
	})
}

// WaitForMessage reads one message from the reader with a timeout.
func WaitForMessage(t *testing.T, r *kafka.Reader, timeout time.Duration) (kafka.Message, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return r.ReadMessage(ctx)
}

// ReadAllAvailable reads all messages currently available (non‑blocking)
// within the given wait duration.
func ReadAllAvailable(r *kafka.Reader, wait time.Duration) ([]kafka.Message, error) {
	var msgs []kafka.Message
	deadline := time.Now().Add(wait)
	for {
		if time.Now().After(deadline) {
			break
		}
		// Use a very short timeout – if no message is available,
		// the call will return almost immediately.
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		msg, err := r.ReadMessage(ctx)
		cancel()
		if err != nil {
			// Timeout or other error → stop collecting
			break
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

// CleanTopic deletes and recreates a Kafka topic via docker exec.
// The topic will have 1 partition and replication-factor 1.
func CleanTopic(t *testing.T, container, topic string) {
    t.Helper()
    run := func(args ...string) {
        cmd := exec.Command("docker", append([]string{"exec", "-i", container}, args...)...)
        // Redirect stderr to discard warnings, but capture output
        _ = cmd.Run() // ignore errors – topic may not exist before creation
    }
    run("kafka-topics", "--bootstrap-server", "localhost:9092", "--delete", "--topic", topic)
    // Give the broker a moment to process the deletion
    time.Sleep(500 * time.Millisecond)
    run("kafka-topics", "--bootstrap-server", "localhost:9092", "--create", "--topic", topic,
        "--partitions", "1", "--replication-factor", "1")
    time.Sleep(500 * time.Millisecond) // allow topic to become available
}