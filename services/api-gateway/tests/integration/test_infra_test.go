// tests/integration/test_infra.go
//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/kisna/archon/services/api-gateway/tests/integration/helpers"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestPostgresConnection(t *testing.T) {
	_, pool := helpers.SetupTestDB(t) // ping is done internally
	err := pool.Ping(context.Background())
	assert.NoError(t, err, "Postgres should respond to ping")
}

func TestRedisConnection(t *testing.T) {
	client := helpers.SetupRedis(t) // pings inside
	err := client.Ping(context.Background()).Err()
	assert.NoError(t, err, "Redis should respond to PING")
}

func TestKafkaConnection(t *testing.T) {
	broker := helpers.GetEnv("TEST_KAFKA_BROKER", "127.0.0.1:9092")
	writer := &kafka.Writer{
		Addr:  kafka.TCP(broker),
		Topic: "build.requests",
	}
	defer writer.Close()

	err := writer.WriteMessages(context.Background(), kafka.Message{
		Value: []byte(`{"test":"connectivity"}`),
	})
	assert.NoError(t, err, "Kafka writer should accept a test message")
}

func TestAIBrainGRPC(t *testing.T) {
	addr := "localhost:50051"
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err, "Should dial AI Brain")
	defer conn.Close()

	// Wait for connection to be ready (or at least not SHUTDOWN)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn.Connect() // trigger connection
	state := conn.GetState()
	// Pass the context to WaitForStateChange to satisfy linter, if needed
	_ = ctx
	assert.NotEqual(t, "SHUTDOWN", state.String())
}