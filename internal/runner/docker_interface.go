package runner

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/build"
)

// DockerClient interface abstracts the docker client methods used by Session.
// This allows mocking for unit tests.
type DockerClient interface {
	CheckDaemon(ctx context.Context) error
	RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error)
	StopContainer(ctx context.Context, containerID string) error
	Exec(ctx context.Context, containerID string, cmd []string) (string, error)
	ExecAsUser(ctx context.Context, containerID string, user string, cmd []string) (string, error)
	ImageExists(ctx context.Context, tag string) (bool, error)
	ImageBuild(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (types.ImageBuildResponse, error)
	PullImage(ctx context.Context, imageRef string) error
}
