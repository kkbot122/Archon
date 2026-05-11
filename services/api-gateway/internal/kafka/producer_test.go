package kafka_test

import (
	"context"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	gatewayKafka "github.com/kisna/archon/services/api-gateway/internal/kafka"
)

// Mock the Kafka Writer
type MockMessageWriter struct {
	mock.Mock
}

func (m *MockMessageWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	args := m.Called(ctx, msgs)
	return args.Error(0)
}

func (m *MockMessageWriter) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestPublishBuildRequest_Success(t *testing.T) {
	mockWriter := new(MockMessageWriter)
	producer := gatewayKafka.NewProducerWithWriter(mockWriter)

	event := gatewayKafka.BuildRequestedEvent{
		ProjectID:   "proj-123",
		VersionHash: "abc",
		ManifestRaw: "{}",
		RequestedAt: time.Now(),
	}

	// We expect the writer to receive a slice of kafka messages
	mockWriter.On("WriteMessages", mock.Anything, mock.AnythingOfType("[]kafka.Message")).Return(nil)

	err := producer.PublishBuildRequest(context.Background(), event)

	assert.NoError(t, err)
	mockWriter.AssertExpectations(t)
}