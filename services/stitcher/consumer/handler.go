package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// BuildRequestedEvent matches the exact schema sent by the API Gateway
type BuildRequestedEvent struct {
	ProjectID   string    `json:"project_id"`
	VersionHash string    `json:"version_hash"`
	ManifestRaw string    `json:"manifest_raw"` // Stored as a string from Gateway
	RequestedAt time.Time `json:"requested_at"`
}

// Orchestrator defines the boundary between the Kafka consumer and the core build logic
type Orchestrator interface {
	ProcessBuild(ctx context.Context, projectID, versionHash, manifestRaw string) error
}

// Handler processes incoming Kafka messages
type Handler struct {
	orchestrator Orchestrator
}

// NewHandler creates a new Kafka message handler
func NewHandler(orch Orchestrator) *Handler {
	return &Handler{
		orchestrator: orch,
	}
}

// HandleMessage parses the Gateway event and triggers the build pipeline
func (h *Handler) HandleMessage(ctx context.Context, payload []byte) error {
	var event BuildRequestedEvent
	
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("failed to parse BuildRequestedEvent: %w", err)
	}

	// Validate required fields
	if event.ProjectID == "" {
		return fmt.Errorf("invalid event: missing project_id")
	}
	if event.ManifestRaw == "" {
		return fmt.Errorf("invalid event: missing manifest_raw")
	}

	log.Printf("🛠️ [Stitcher] Received build request for Project: %s (Version: %s)", event.ProjectID, event.VersionHash)

	// Hand off to the core build orchestrator
	err := h.orchestrator.ProcessBuild(ctx, event.ProjectID, event.VersionHash, event.ManifestRaw)
	if err != nil {
		return fmt.Errorf("build process failed for project %s: %w", event.ProjectID, err)
	}

	return nil
}