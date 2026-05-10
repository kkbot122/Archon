package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// Workspace represents an isolated build directory on disk
type Workspace struct {
	BuildID string
	Path    string
}

// Create generates a secure, isolated temporary directory for the build
func Create(buildID string) (*Workspace, error) {
	// e.g., /tmp/archon-builds/abc-123
	dirPath := filepath.Join(os.TempDir(), "archon-builds", buildID)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	return &Workspace{
		BuildID: buildID,
		Path:    dirPath,
	}, nil
}

// Cleanup permanently deletes the workspace and its contents
func (w *Workspace) Cleanup() error {
	if w.Path == "" {
		return nil
	}
	return os.RemoveAll(w.Path)
}