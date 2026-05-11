package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Validate ensures the manifest has the minimum required fields before we try to build it.
func Validate(m *Manifest) error {
	if m.Metadata.ProjectName == "" {
		return fmt.Errorf("manifest is missing project name")
	}
	if len(m.Nodes) == 0 {
		return fmt.Errorf("manifest has no nodes to build")
	}

	// Ensure every node has an ID and a Type
	for i, node := range m.Nodes {
		if node.ID == "" {
			return fmt.Errorf("node at index %d is missing an ID", i)
		}
		if node.Type == "" {
			return fmt.Errorf("node '%s' is missing a Type", node.ID)
		}
	}

	return nil
}

// GenerateIdempotencyKey creates a stable hash of the manifest to prevent duplicate builds.
func GenerateIdempotencyKey(m *Manifest) (string, error) {
	// By marshalling our tightly controlled struct, we guarantee consistent JSON ordering
	bytes, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("failed to hash manifest: %w", err)
	}

	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:]), nil
}