// internal/kafka/events.go
package kafka

import "time"

// Topic names
const (
	TopicBuildRequested = "architect.build.requested"
	TopicBuildComplete  = "architect.build.complete"
	TopicBuildFailed    = "architect.build.failed"
)

// BuildRequestedEvent is the payload dropped into Kafka when a user clicks "Ship"
type BuildRequestedEvent struct {
	ProjectID   string    `json:"project_id"`
	VersionHash string    `json:"version_hash"`
	ManifestRaw string    `json:"manifest_raw"` // The JSON blueprint
	RequestedAt time.Time `json:"requested_at"`
}