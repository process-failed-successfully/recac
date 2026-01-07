package docker

import (
	"context"

	"github.com/docker/docker/api/types"
)

// IClient defines the interface for the high-level Docker client.
// This allows for mocking in tests.
type IClient interface {
	ServerVersion(ctx context.Context) (types.Version, error)
	Close() error
	CheckDaemon(ctx context.Context) error
	CheckSocket(ctx context.Context) error
	CheckImage(ctx context.Context, imageRef string) (bool, error)
	PullImage(ctx context.Context, imageRef string) error
	ImageBuild(ctx context.Context, opts ImageBuildOptions) (string, error)
	RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error)
	StopContainer(ctx context.Context, containerID string) error
	Exec(ctx context.Context, containerID string, cmd []string) (string, error)
	ExecAsUser(ctx context.Context, containerID string, user string, cmd []string) (string, error)
	ImageExists(ctx context.Context, tag string) (bool, error)
}
