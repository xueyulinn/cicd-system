package worker

import (
	"context"
	"fmt"

	"github.com/moby/moby/client"
)

// NewDockerClient creates a Docker API client configured from environment variables
// (DOCKER_HOST, DOCKER_API_VERSION, etc.). Use Close() when done.
// WithAPIVersionNegotiation allows the client to negotiate API version with the daemon.
func NewDockerClient(ctx context.Context) (*client.Client, error) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, err
	}
	// Verify connection to the daemon
	if _, err := cli.Ping(ctx, client.PingOptions{}); err != nil {
		_ = cli.Close()
		return nil, err
	}
	return cli, nil
}

// PingDocker verifies that the Docker daemon is reachable using cli.
func PingDocker(ctx context.Context, cli *client.Client) error {
	if cli == nil {
		return fmt.Errorf("docker client not available")
	}

	if _, err := cli.Ping(ctx, client.PingOptions{}); err != nil {
		return err
	}

	return nil
}
