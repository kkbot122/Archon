// tests/integration/helpers/setup.go
package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/kisna/archon/services/api-gateway/internal/db"
	"github.com/stretchr/testify/require"
)

// getEnv fetches an environment variable or returns a default value
func GetEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// SetupTestDB connects to the test database, runs migrations, and returns a repository.
// It truncates all tables between tests via t.Cleanup.
func SetupTestDB(t *testing.T) (*db.Repository, *pgxpool.Pool) {
	dbURL := GetEnv("TEST_DATABASE_URL", "postgres://archon_user:archon_password@localhost:5433/archon_db?sslmode=disable")
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		t.Fatalf("Failed to parse database URL: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	// Run schema (assumes schema.sql is at deploy/docker/init.sql or similar)
	schema, err := os.ReadFile("../../deploy/docker/init.sql")
	if err == nil {
		_, err = pool.Exec(context.Background(), string(schema))
		if err != nil {
			t.Fatalf("Failed to execute schema: %v", err)
		}
	}
	repo := db.NewRepository(pool)

	// Cleanup: truncate tables after each test
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "TRUNCATE TABLE manifests, projects, users CASCADE")
		pool.Close()
	})
	return repo, pool
}

// SetupRedis creates a Redis client and pings it.
func SetupRedis(t *testing.T) *redis.Client {
	addr := GetEnv("TEST_REDIS_ADDR", "localhost:6379")
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	// Verify connectivity
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("Redis ping failed: %v", err)
	}
	t.Cleanup(func() {
		client.Close()
	})
	return client
}

// SetupKafkaConsumer creates a Kafka reader and returns a channel that receives raw message values.
func SetupKafkaConsumer(t *testing.T, topic string) (chan []byte, *kafka.Reader) {
	broker := GetEnv("TEST_KAFKA_BROKER", "127.0.0.1:9092")
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{broker},
		Topic:   topic,
		GroupID: "test-consumer-" + t.Name(),
		MaxWait: time.Second,
	})
	ch := make(chan []byte, 10)
	go func() {
		for {
			msg, err := reader.ReadMessage(context.Background())
			if err != nil {
				log.Printf("Kafka consumer error: %v", err)
				return
			}
			ch <- msg.Value
		}
	}()
	t.Cleanup(func() {
		reader.Close()
	})
	return ch, reader
}

// WaitForKafkaMessage waits up to timeout for a message to arrive on a channel.
func WaitForKafkaMessage(ch chan []byte, timeout time.Duration) ([]byte, error) {
	select {
	case msg := <-ch:
		return msg, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for Kafka message")
	}
}

// WaitForKafkaMessageContaining waits for a message that contains substr.
func WaitForKafkaMessageContaining(ch chan []byte, substr string, timeout time.Duration) ([]byte, error) {
    deadline := time.After(timeout)
    for {
        select {
        case msg := <-ch:
            if strings.Contains(string(msg), substr) {
                return msg, nil
            }
            // Discard messages that don't match
        case <-deadline:
            return nil, fmt.Errorf("timeout waiting for Kafka message containing %q", substr)
        }
    }
}

// SetupWebSocketClient connects to the WebSocket endpoint for a given project.
func SetupWebSocketClient(t *testing.T, projectID string) *websocket.Conn {
	u := fmt.Sprintf("ws://localhost:4000/ws?projectId=%s", projectID)
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	t.Cleanup(func() {
		conn.Close()
	})
	return conn
}

// MakeGraphQLRequest performs a GraphQL query against the API Gateway.
func MakeGraphQLRequest(t *testing.T, query string, variables map[string]interface{}) (map[string]interface{}, error) {
	url := GetEnv("TEST_GRAPHQL_URL", "http://localhost:4000/query")
	body := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", url, strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer valid-mock-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if errStr, ok := result["errors"]; ok {
		return nil, fmt.Errorf("GraphQL error: %v", errStr)
	}
	return result, nil
}

// CreateTestProject creates a project via GraphQL and returns the projectID.
func CreateTestProject(t *testing.T) string {
	query := `mutation { createProject(name: "IntegrationTestProject") { id name } }`
	res, err := MakeGraphQLRequest(t, query, nil)
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}
	data := res["data"].(map[string]interface{})
	project := data["createProject"].(map[string]interface{})
	return project["id"].(string)
}

// MustPublishKafkaEvent writes an event to a Kafka topic.
func MustPublishKafkaEvent(t *testing.T, topic string, event interface{}) {
	broker := GetEnv("TEST_KAFKA_BROKER", "127.0.0.1:9092")
	writer := &kafka.Writer{
		Addr:  kafka.TCP(broker),
		Topic: topic,
	}
	defer writer.Close()

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}
	if err := writer.WriteMessages(context.Background(), kafka.Message{Value: payload}); err != nil {
		t.Fatalf("Failed to write to Kafka: %v", err)
	}
}

// EnsureDummyUser inserts the mock user used by the gateway.
// This must be called before any test that relies on the user existing.
func EnsureDummyUser(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, email) VALUES ('11111111-1111-1111-1111-111111111111', 'mock@archon.dev') ON CONFLICT DO NOTHING`,
	)
	require.NoError(t, err, "failed to ensure dummy user exists")
}

// SetupKafkaConsumerFromLatest creates a Kafka reader that only picks up messages
// written after the consumer starts (StartOffset = LastOffset).
func SetupKafkaConsumerFromLatest(t *testing.T, topic string) (chan []byte, *kafka.Reader) {
	broker := GetEnv("TEST_KAFKA_BROKER", "127.0.0.1:9092")
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{broker},
		Topic:       topic,
		GroupID:     "test-consumer-" + t.Name(),
		StartOffset: kafka.LastOffset, // <-- only new messages
		MaxWait:     time.Second,
	})
	ch := make(chan []byte, 10)
	go func() {
		for {
			msg, err := reader.ReadMessage(context.Background())
			if err != nil {
				log.Printf("Kafka consumer error: %v", err)
				return
			}
			ch <- msg.Value
		}
	}()
	t.Cleanup(func() {
		reader.Close()
	})
	return ch, reader
}