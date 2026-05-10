package manifest

import (
	"fmt"
	"strings"
)

// Validator defines the interface for checking manifest integrity
type Validator interface {
	Validate(m *ProjectManifest) error
}

type DefaultValidator struct{}

func NewValidator() *DefaultValidator {
	return &DefaultValidator{}
}

func (v *DefaultValidator) Validate(m *ProjectManifest) error {
	if m == nil {
		return fmt.Errorf("manifest is nil")
	}

	if strings.TrimSpace(m.Metadata.ProjectName) == "" {
		return fmt.Errorf("project_name is missing")
	}

	if len(m.Nodes) == 0 {
		return fmt.Errorf("manifest contains no nodes to build")
	}

	// Ensure all nodes have unique IDs
	seenIDs := make(map[string]bool)
	for _, node := range m.Nodes {
		if node.ID == "" {
			return fmt.Errorf("node missing ID")
		}
		if seenIDs[node.ID] {
			return fmt.Errorf("duplicate node ID found: %s", node.ID)
		}
		seenIDs[node.ID] = true
	}

	// Validate connections reference valid nodes
	for _, conn := range m.Connections {
		if !seenIDs[conn.SourceID] {
			return fmt.Errorf("connection source_id %s does not exist", conn.SourceID)
		}
		if !seenIDs[conn.TargetID] {
			return fmt.Errorf("connection target_id %s does not exist", conn.TargetID)
		}
	}

	return nil
}