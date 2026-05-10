package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type Metadata struct {
	ProjectName   string `json:"project_name"`
	TargetCloud   string `json:"target_cloud"`
	SchemaVersion string `json:"schema_version"`
}

type Node struct {
	ID      string            `json:"id"`
	Type    string            `json:"type"`
	Version string            `json:"version"`
	Config  map[string]string `json:"config"`
}

type Connection struct {
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
	Protocol string `json:"protocol"`
}

type ProjectManifest struct {
	Metadata     Metadata     `json:"metadata"`
	Nodes        []Node       `json:"nodes"`
	Connections  []Connection `json:"connections"`
	FeatureFlags []string     `json:"feature_flags"`
}

// Parse extracts the manifest from a raw JSON byte slice
func Parse(raw []byte) (*ProjectManifest, error) {
	var m ProjectManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}
	return &m, nil
}

// GenerateIdempotencyKey creates the dedup key: sha256(project_id + manifest_hash)
func GenerateIdempotencyKey(projectID string, manifestHash string) string {
	hash := sha256.New()
	hash.Write([]byte(projectID + manifestHash))
	return hex.EncodeToString(hash.Sum(nil))
}