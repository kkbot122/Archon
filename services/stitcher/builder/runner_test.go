package builder

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/kisna/archon/services/stitcher/workspace"
	"github.com/stretchr/testify/assert"
)

func TestShadowBuilder(t *testing.T) {
	b, err := New()
	assert.NoError(t, err)

	// Create an isolated workspace
	ws, _ := workspace.Create("test-shadow-build-123")
	defer ws.Cleanup()

	// Write a tiny script into the workspace
	scriptPath := ws.Path + "/test.sh"
	err = os.WriteFile(scriptPath, []byte("#!/bin/sh\necho 'Hello from Archon Shadow Builder!'\nexit 0"), 0755)
	assert.NoError(t, err)

	// Set a hard 30-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run an Alpine container, mount the workspace, and execute our script
	result, err := b.RunBuild(ctx, "test-shadow-build-123", ws, "alpine:latest", []string{"/bin/sh", "/workspace/test.sh"})
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	
	// Verify exact outcomes
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Logs, "Hello from Archon Shadow Builder!")
}