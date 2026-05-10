package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kisna/archon/services/stitcher/builder"
	"github.com/kisna/archon/services/stitcher/internal/metrics"
	"github.com/kisna/archon/services/stitcher/manifest"
	"github.com/kisna/archon/services/stitcher/publisher"
	"github.com/kisna/archon/services/stitcher/stitcher"
	"github.com/kisna/archon/services/stitcher/workspace"
	"github.com/rs/zerolog/log"
)

type Handler struct {
	validator *manifest.DefaultValidator
	engine    *stitcher.Engine
	docker    *builder.DockerBuilder
	pub       *publisher.Publisher
}

func NewHandler(v *manifest.DefaultValidator, e *stitcher.Engine, d *builder.DockerBuilder, p *publisher.Publisher) *Handler {
	return &Handler{
		validator: v,
		engine:    e,
		docker:    d,
		pub:       p,
	}
}

// HandleMessage orchestrates the entire lifecycle of a single build request
func (h *Handler) HandleMessage(ctx context.Context, rawPayload []byte) error {
	// 1. Unmarshal the incoming request (Assuming the payload wraps the manifest and metadata)
	var req struct {
		TraceID     string          `json:"trace_id"`
		ProjectID   string          `json:"project_id"`
		VersionHash string          `json:"version_hash"`
		Manifest    json.RawMessage `json:"manifest"`
	}
	if err := json.Unmarshal(rawPayload, &req); err != nil {
		return fmt.Errorf("failed to decode build request: %w", err)
	}

	logger := log.With().Str("project_id", req.ProjectID).Str("trace_id", req.TraceID).Logger()
	logger.Info().Msg("Starting build pipeline")

	// 2. Parse and Validate the Manifest
	m, err := manifest.Parse(req.Manifest)
	if err != nil {
		h.reportFailure(ctx, req.TraceID, req.ProjectID, req.VersionHash, "Manifest Parse Error", err.Error(), "")
		return err
	}

	if err := h.validator.Validate(m); err != nil {
		h.reportFailure(ctx, req.TraceID, req.ProjectID, req.VersionHash, "Manifest Validation Error", err.Error(), "")
		return err
	}

	// 3. Create Idempotency Key & Workspace
	buildID := manifest.GenerateIdempotencyKey(req.ProjectID, req.VersionHash)
	ws, err := workspace.Create(buildID)
	if err != nil {
		return fmt.Errorf("workspace creation failed: %w", err)
	}
	// CRITICAL: Ensure workspace is wiped off the disk when this function exits
	defer ws.Cleanup()

	// 4. Stitch the Code
	logger.Info().Msg("Stitching files from Atomic Library...")
	if err := h.engine.Stitch(ws, m); err != nil {
		h.reportFailure(ctx, req.TraceID, req.ProjectID, req.VersionHash, buildID, "Stitcher Error: "+err.Error(), "")
		return err
	}

	// 5. Run the Shadow Build
	logger.Info().Msg("Executing Shadow Build...")
	// We'll use a generic node image for the shadow build for now, this can be dynamic later
	result, err := h.docker.RunBuild(ctx, buildID, ws, "node:20-alpine", []string{"npm", "install"})
	if err != nil {
		h.reportFailure(ctx, req.TraceID, req.ProjectID, req.VersionHash, buildID, "Docker Execution Error: "+err.Error(), "")
		return err
	}

	// 6. Evaluate Result & Publish
	if !result.Success {
		h.reportFailure(ctx, req.TraceID, req.ProjectID, req.VersionHash, buildID, "Shadow Build Failed (Non-Zero Exit Code)", result.Logs)
		return fmt.Errorf("shadow build failed")
	}

	logger.Info().Msg("Build pipeline completed successfully")
	metrics.BuildsTotal.WithLabelValues("success").Inc()
	return h.pub.PublishSuccess(ctx, "architect.build.completed", publisher.BuildCompletedEvent{
		TraceID:     req.TraceID,
		ProjectID:   req.ProjectID,
		BuildID:     buildID,
		VersionHash: req.VersionHash,
		CompletedAt: time.Now(),
		Logs:        result.Logs,
	})
}

func (h *Handler) reportFailure(ctx context.Context, traceID, projectID, hash, buildID, errMsg, logs string) {
	metrics.BuildsTotal.WithLabelValues("error").Inc()
	_ = h.pub.PublishFailure(ctx, "architect.build.failed", publisher.BuildFailedEvent{
		TraceID:     traceID,
		ProjectID:   projectID,
		BuildID:     buildID,
		VersionHash: hash,
		FailedAt:    time.Now(),
		Error:       errMsg,
		Logs:        logs,
	})
}