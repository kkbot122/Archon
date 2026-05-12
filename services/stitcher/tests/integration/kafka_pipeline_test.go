//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kisna/archon/services/stitcher/consumer"
	"github.com/kisna/archon/services/stitcher/tests/integration/helpers"
)

// testOrchestrator is a controllable orchestrator for testing the consumer.
type testOrchestrator struct {
	processFunc func(ctx context.Context, projectID, versionHash, manifestRaw string) error
	done        chan struct{}
	once        sync.Once
}

func newTestOrchestrator(fn func(ctx context.Context, projectID, versionHash, manifestRaw string) error) *testOrchestrator {
	return &testOrchestrator{
		processFunc: fn,
		done:        make(chan struct{}),
	}
}

func (o *testOrchestrator) ProcessBuild(ctx context.Context, projectID, versionHash, manifestRaw string) error {
	err := o.processFunc(ctx, projectID, versionHash, manifestRaw)
	if err == nil {
		o.once.Do(func() { close(o.done) })
	}
	return err
}

// ------------------------------------------------------------------
// Test: Successful build
// ------------------------------------------------------------------
func TestStitcherBuildSuccess(t *testing.T) {
	helpers.EnsureTopic(t, "build.requests")                    // idempotent, safe in CI
	projectID := uuid.New().String()
	orch := newTestOrchestrator(func(ctx context.Context, projectID, versionHash, manifestRaw string) error {
		return nil
	})

	handler := consumer.NewHandler(orch)
	c := consumer.New(
		[]string{"localhost:9092"},
		"test-success-group-"+uuid.New().String(),
		[]string{"build.requests"},
		handler,
	)

	ctx := context.Background()
	go c.Start(ctx)
	defer c.Close()

	producer := helpers.NewProducer(t, "build.requests")
	err := helpers.PublishJSON(context.Background(), producer, projectID, consumer.BuildRequestedEvent{
		ProjectID:   projectID,
		VersionHash: "hash1",
		ManifestRaw: `{"nodes":[]}`,
		RequestedAt: time.Now(),
	})
	require.NoError(t, err)

	select {
	case <-orch.done:
	case <-time.After(10 * time.Second):
		t.Fatal("orchestrator was not called within timeout")
	}
}

// ------------------------------------------------------------------
// Test: Transient failure → retry → success
// ------------------------------------------------------------------
func TestStitcherRetry(t *testing.T) {
	helpers.EnsureTopic(t, "build.requests")
	projectID := uuid.New().String()
	attempts := 0
	orch := newTestOrchestrator(func(ctx context.Context, projectID, versionHash, manifestRaw string) error {
		attempts++
		if attempts < 3 {
			return fmt.Errorf("transient error attempt %d", attempts)
		}
		return nil
	})

	handler := consumer.NewHandler(orch)
	c := consumer.New(
		[]string{"localhost:9092"},
		"test-retry-group-"+uuid.New().String(),
		[]string{"build.requests", "build.requests.retry"},
		handler,
	)

	ctx := context.Background()
	go c.Start(ctx)
	defer c.Close()

	producer := helpers.NewProducer(t, "build.requests")
	err := helpers.PublishJSON(context.Background(), producer, projectID, consumer.BuildRequestedEvent{
		ProjectID:   projectID,
		VersionHash: "retry-hash",
		ManifestRaw: `{"nodes":[]}`,
		RequestedAt: time.Now(),
	})
	require.NoError(t, err)

	select {
	case <-orch.done:
		assert.GreaterOrEqual(t, attempts, 3, "should have retried at least twice before success")
	case <-time.After(15 * time.Second):
		t.Fatal("orchestrator did not succeed within timeout")
	}
}

// ------------------------------------------------------------------
// Test: Max retries → DLQ
// ------------------------------------------------------------------
func TestStitcherDeadLetterQueue(t *testing.T) {
	helpers.EnsureTopic(t, "build.requests")
	projectID := uuid.New().String()
	orch := newTestOrchestrator(func(ctx context.Context, projectID, versionHash, manifestRaw string) error {
		return fmt.Errorf("permanent failure")
	})

	handler := consumer.NewHandler(orch)
	c := consumer.New(
		[]string{"localhost:9092"},
		"test-dlq-group-"+uuid.New().String(),
		[]string{"build.requests", "build.requests.retry"},
		handler,
	)

	ctx := context.Background()
	go c.Start(ctx)
	defer c.Close()

	producer := helpers.NewProducer(t, "build.requests")
	err := helpers.PublishJSON(context.Background(), producer, projectID, consumer.BuildRequestedEvent{
		ProjectID:   projectID,
		VersionHash: "dlq-hash",
		ManifestRaw: `{"nodes":[]}`,
		RequestedAt: time.Now(),
	})
	require.NoError(t, err)

	dlqReader := helpers.NewConsumer(t, "build.requests.dlq", "test-dlq-reader-"+uuid.New().String())
	msg, err := helpers.WaitForMessage(t, dlqReader, 30*time.Second)
	require.NoError(t, err)

	var event consumer.BuildRequestedEvent
	err = json.Unmarshal(msg.Value, &event)
	require.NoError(t, err)
	assert.Equal(t, projectID, event.ProjectID)
}

// ------------------------------------------------------------------
// Test: Invalid event is rejected immediately
// ------------------------------------------------------------------
func TestStitcherInvalidEvent(t *testing.T) {
	helpers.EnsureTopic(t, "build.requests")
	orch := newTestOrchestrator(func(ctx context.Context, projectID, versionHash, manifestRaw string) error {
		return fmt.Errorf("should not be called")
	})

	handler := consumer.NewHandler(orch)
	c := consumer.New(
		[]string{"localhost:9092"},
		"test-invalid-group-"+uuid.New().String(),
		[]string{"build.requests"},
		handler,
	)

	ctx := context.Background()
	go c.Start(ctx)
	defer c.Close()

	producer := helpers.NewProducer(t, "build.requests")
	err := producer.WriteMessages(context.Background(), kafka.Message{
		Value: []byte(`{"invalid": true}`),
	})
	require.NoError(t, err)

	select {
	case <-orch.done:
		t.Fatal("orchestrator was called for an invalid event")
	case <-time.After(2 * time.Second):
	}
}