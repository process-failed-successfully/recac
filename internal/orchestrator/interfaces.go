package orchestrator

import "context"

// DockerClient abstraction for testing
type DockerClient interface {
	RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error)
	Exec(ctx context.Context, containerID string, cmd []string) (string, error)
}
