package builder

import (
	"bytes"
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// getContainerLogs securely fetches and demultiplexes stdout/stderr from the container
func getContainerLogs(ctx context.Context, cli *client.Client, containerID string) (string, error) {
	// v24 specific struct
	options := types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true}
	
	out, err := cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}
	defer out.Close()

	var stdout, stderr bytes.Buffer
	// stdcopy is Docker's official demultiplexer for container logs
	_, err = stdcopy.StdCopy(&stdout, &stderr, out)
	if err != nil {
		return "", fmt.Errorf("failed to demultiplex logs: %w", err)
	}

	return fmt.Sprintf("=== STDOUT ===\n%s\n=== STDERR ===\n%s", stdout.String(), stderr.String()), nil
}