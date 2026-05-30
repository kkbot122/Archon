// internal/kafka/events.go
package kafka

import "time"

// Topic names — single source of truth for all services.
// Both the API Gateway and the Stitcher must import these constants;
// never use raw string literals for topic names anywhere else.
const (
	TopicBuildRequests = "build.requests"
	TopicBuildRetry    = "build.requests.retry"
	TopicBuildDLQ      = "build.requests.dlq"
	TopicBuildStatus   = "build.status"
)

// BuildRequestedEvent is the payload dropped into Kafka when a user clicks "Ship"
type BuildRequestedEvent struct {
	ProjectID   string    `json:"project_id"`
	VersionHash string    `json:"version_hash"`
	ManifestRaw string    `json:"manifest_raw"` // The JSON blueprint
	RequestedAt time.Time `json:"requested_at"`
}