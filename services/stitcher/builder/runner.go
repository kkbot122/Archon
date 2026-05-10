package builder

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/kisna/archon/services/stitcher/internal/metrics"
	"github.com/kisna/archon/services/stitcher/workspace"
	"github.com/rs/zerolog/log"
)

type BuildResult struct {
	Success  bool
	ExitCode int
	Logs     string
}

// RunBuild executes the stitched code inside an ephemeral container
func (b *DockerBuilder) RunBuild(ctx context.Context, buildID string, ws *workspace.Workspace, imageName string, cmd []string) (*BuildResult, error) {
	start := time.Now()
	logger := log.With().Str("build_id", buildID).Str("stage", "shadow_build").Logger()

	logger.Info().Msgf("Pulling image %s (if not exists)...", imageName)
	
	// v24 specific struct
	reader, err := b.cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err == nil {
		io.Copy(io.Discard, reader)
		reader.Close()
	}

	logger.Info().Msg("Creating ephemeral container...")
	resp, err := b.cli.ContainerCreate(ctx, &container.Config{
		Image:      imageName,
		Cmd:        cmd,
		WorkingDir: "/workspace",
		Tty:        false,
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: ws.Path,
				Target: "/workspace",
			},
		},
	}, nil, nil, "")

	if err != nil {
		metrics.DockerErrors.Inc()
		return nil, fmt.Errorf("failed to create shadow build container: %w", err)
	}

	containerID := resp.ID

	// CRITICAL CONSTRAINT: Always remove the container in a defer
	defer func() {
		logger.Info().Msg("Tearing down ephemeral container...")
		// v24 specific struct
		removeOptions := types.ContainerRemoveOptions{Force: true, RemoveVolumes: true}
		if err := b.cli.ContainerRemove(context.Background(), containerID, removeOptions); err != nil {
			logger.Error().Err(err).Msg("Failed to remove shadow build container (Zombie Warning)")
		}
	}()

	// Start the container
	// v24 specific struct
	if err := b.cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Wait for it to finish or timeout via Context
	statusCh, errCh := b.cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	
	var exitCode int
	select {
	case err := <-errCh:
		if err != nil {
			return nil, fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		exitCode = int(status.StatusCode)
	case <-ctx.Done():
		return nil, fmt.Errorf("build timed out or cancelled: %w", ctx.Err())
	}

	// Fetch logs
	logsStr, err := getContainerLogs(ctx, b.cli, containerID)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to fetch container logs")
	}

	duration := time.Since(start).Milliseconds()
	success := exitCode == 0

	metrics.BuildDuration.Observe(float64(duration) / 1000.0)

	// Structured Logging metric
	logger.Info().
		Bool("success", success).
		Int("exit_code", exitCode).
		Int64("duration_ms", duration).
		Msg("Shadow build completed")

	return &BuildResult{
		Success:  success,
		ExitCode: exitCode,
		Logs:     logsStr,
	}, nil
}