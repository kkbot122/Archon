package builder

import (
	"fmt"
	"github.com/docker/docker/client"
)

type DockerBuilder struct {
	cli *client.Client
}

// New initializes a new Docker client from the environment (DOCKER_HOST, etc.)
func New() (*DockerBuilder, error) {
	// Negotiates the API version automatically with the Docker Daemon
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize docker client: %w", err)
	}
	return &DockerBuilder{cli: cli}, nil
}