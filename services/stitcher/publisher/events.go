package publisher

import "time"

// BuildCompletedEvent is published when the Shadow Build exits with code 0
type BuildCompletedEvent struct {
	TraceID     string    `json:"trace_id"`
	ProjectID   string    `json:"project_id"`
	BuildID     string    `json:"build_id"`
	VersionHash string    `json:"version_hash"`
	CompletedAt time.Time `json:"completed_at"`
	Logs        string    `json:"logs"`
}

// BuildFailedEvent is published when validation, stitching, or the Shadow Build fails
type BuildFailedEvent struct {
	TraceID     string    `json:"trace_id"`
	ProjectID   string    `json:"project_id"`
	BuildID     string    `json:"build_id"`
	VersionHash string    `json:"version_hash"`
	FailedAt    time.Time `json:"failed_at"`
	Error       string    `json:"error"`
	Logs        string    `json:"logs"` // Might be empty if it failed before Docker
}