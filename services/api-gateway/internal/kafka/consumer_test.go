package kafka_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	gatewayKafka "github.com/kisna/archon/services/api-gateway/internal/kafka"
)

// 1. Mock the Redis Publisher boundary
type MockRedisPublisher struct {
	mock.Mock
}

func (m *MockRedisPublisher) PublishBuildUpdate(ctx context.Context, projectID string, payload string) error {
	args := m.Called(ctx, projectID, payload)
	return args.Error(0)
}

func TestHandleBuildEvent_Success(t *testing.T) {
	mockRedis := new(MockRedisPublisher)
	
	// 2. Instantiate our Gateway Consumer
	consumer := gatewayKafka.NewGatewayConsumer(mockRedis)

	// 3. The fake raw JSON event coming from the Stitcher service via Kafka
	kafkaMessage := []byte(`{"project_id": "123e4567-e89b-12d3-a456-426614174000", "status": "BUILD_SUCCESS", "url": "https://archon.dev/app"}`)

	// 4. We expect the consumer to extract the project_id and pass the raw payload to Redis
	mockRedis.On("PublishBuildUpdate", mock.Anything, "123e4567-e89b-12d3-a456-426614174000", string(kafkaMessage)).Return(nil)

	// 5. Execute the handler
	err := consumer.HandleMessage(context.Background(), kafkaMessage)

	// 6. Verify
	assert.NoError(t, err)
	mockRedis.AssertExpectations(t)
}

func TestHandleBuildEvent_MissingProjectID(t *testing.T) {
	mockRedis := new(MockRedisPublisher)
	consumer := gatewayKafka.NewGatewayConsumer(mockRedis)

	// Bad payload missing project_id
	badMessage := []byte(`{"status": "BUILD_SUCCESS"}`)

	err := consumer.HandleMessage(context.Background(), badMessage)

	// It should error out and NEVER call Redis
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing project_id")
	mockRedis.AssertNotCalled(t, "PublishBuildUpdate")
}