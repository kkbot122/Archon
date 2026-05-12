//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kisna/archon/services/stitcher/builder"
	"github.com/kisna/archon/services/stitcher/workspace"
)

func TestShadowBuildSuccess(t *testing.T) {
	b, err := builder.New()
	require.NoError(t, err, "docker daemon must be running")

	ws, err := workspace.Create("shadow-test-success")
	require.NoError(t, err)
	defer ws.Cleanup()

	// Write a dummy file into the workspace
	err = os.WriteFile(ws.Path+"/testfile", []byte("hello archon\n"), 0644)
	require.NoError(t, err)

	result, err := b.RunBuild(
		context.Background(),
		"shadow-test-success",
		ws,
		"alpine:latest",
		[]string{"cat", "/workspace/testfile"},
	)
	require.NoError(t, err)
	assert.True(t, result.Success, "exit code should be 0")
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Logs, "hello archon")
}

func TestShadowBuildFailedExitCode(t *testing.T) {
	b, err := builder.New()
	require.NoError(t, err)

	ws, err := workspace.Create("shadow-test-fail")
	require.NoError(t, err)
	defer ws.Cleanup()

	result, err := b.RunBuild(
		context.Background(),
		"shadow-test-fail",
		ws,
		"alpine:latest",
		[]string{"sh", "-c", "exit 1"},
	)
	require.NoError(t, err)
	assert.False(t, result.Success, "exit code should be non-zero")
	assert.Equal(t, 1, result.ExitCode)
}

func TestShadowBuildTimeout(t *testing.T) {
	b, err := builder.New()
	require.NoError(t, err)

	ws, err := workspace.Create("shadow-test-timeout")
	require.NoError(t, err)
	defer ws.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err = b.RunBuild(
		ctx,
		"shadow-test-timeout",
		ws,
		"alpine:latest",
		[]string{"sleep", "10"},
	)
	assert.Error(t, err, "a timeout error is expected")
}

// TestDockerBuilderFailsGracefully verifies that a build fails with a
// connection error when the Docker daemon is unreachable.  Setting
// DOCKER_HOST alone does NOT cause builder.New() to fail; the error
// appears on the first actual operation.
func TestDockerBuilderFailsGracefully(t *testing.T) {
	os.Setenv("DOCKER_HOST", "tcp://127.0.0.1:99999")
	defer os.Unsetenv("DOCKER_HOST")

	b, err := builder.New()
	require.NoError(t, err, "client handle always succeeds; failure occurs at operation time")

	ws, err := workspace.Create("shadow-test-down")
	require.NoError(t, err)
	defer ws.Cleanup()

	_, err = b.RunBuild(
		context.Background(),
		"shadow-test-down",
		ws,
		"alpine:latest",
		[]string{"echo", "hello"},
	)
	assert.Error(t, err, "expected a connection error when daemon is unreachable")
}