package consumer_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/kisna/archon/services/stitcher/consumer" 
)

// MockOrchestrator mocks the core Stitcher build process
type MockOrchestrator struct {
	mock.Mock
}

func (m *MockOrchestrator) ProcessBuild(ctx context.Context, projectID, versionHash, manifestRaw string) error {
	args := m.Called(ctx, projectID, versionHash, manifestRaw)
	return args.Error(0)
}

func TestHandleMessage_Success(t *testing.T) {
	mockOrch := new(MockOrchestrator)
	handler := consumer.NewHandler(mockOrch)

	// Exact JSON payload schema sent by the Gateway
	validPayload := []byte(`{
		"project_id": "proj-999",
		"version_hash": "abc-123",
		"manifest_raw": "{\"nodes\": []}",
		"requested_at": "2026-05-11T14:00:00Z"
	}`)

	// We expect the handler to extract the fields and pass them to the orchestrator
	mockOrch.On("ProcessBuild", mock.Anything, "proj-999", "abc-123", "{\"nodes\": []}").Return(nil)

	err := handler.HandleMessage(context.Background(), validPayload)

	assert.NoError(t, err)
	mockOrch.AssertExpectations(t)
}

func TestHandleMessage_MissingProjectID(t *testing.T) {
	mockOrch := new(MockOrchestrator)
	handler := consumer.NewHandler(mockOrch)

	// Payload missing the project_id
	badPayload := []byte(`{
		"version_hash": "abc-123",
		"manifest_raw": "{}"
	}`)

	err := handler.HandleMessage(context.Background(), badPayload)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing project_id")
	mockOrch.AssertNotCalled(t, "ProcessBuild")
}