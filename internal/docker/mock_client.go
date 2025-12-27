package docker

import (
	"context"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// MockAPI implements APIClient for testing and mock execution.
type MockAPI struct {
	PingFunc                func(ctx context.Context) (types.Ping, error)
	ImagePullFunc           func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
	ImageBuildFunc          func(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (types.ImageBuildResponse, error)
	ContainerCreateFunc     func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error)
	ContainerStartFunc      func(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerExecCreateFunc func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error)
	ContainerExecAttachFunc func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error)
	ContainerStopFunc       func(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemoveFunc     func(ctx context.Context, containerID string, options container.RemoveOptions) error
	CloseFunc               func() error
}

func (m *MockAPI) Ping(ctx context.Context) (types.Ping, error) {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return types.Ping{}, nil
}

func (m *MockAPI) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	if m.ImagePullFunc != nil {
		return m.ImagePullFunc(ctx, ref, options)
	}
	// Return empty reader
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *MockAPI) ImageBuild(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (types.ImageBuildResponse, error) {
	if m.ImageBuildFunc != nil {
		return m.ImageBuildFunc(ctx, buildContext, options)
	}
	// Return mock successful build response
	mockStream := `{"stream":"Step 1/2 : FROM alpine\n"}
{"stream":" ---> abc123def456\n"}
{"stream":"Step 2/2 : RUN echo hello\n"}
{"stream":" ---> Running in container123\n"}
{"aux":{"ID":"sha256:mockimageid123456789"}}
{"stream":"Successfully built mockimageid123456789\n"}`
	return types.ImageBuildResponse{
		Body: io.NopCloser(strings.NewReader(mockStream)),
	}, nil
}

func (m *MockAPI) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
	if m.ContainerCreateFunc != nil {
		return m.ContainerCreateFunc(ctx, config, hostConfig, networkingConfig, platform, containerName)
	}
	return container.CreateResponse{ID: "mock-container-id"}, nil
}

func (m *MockAPI) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	if m.ContainerStartFunc != nil {
		return m.ContainerStartFunc(ctx, containerID, options)
	}
	return nil
}

func (m *MockAPI) ContainerExecCreate(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
	if m.ContainerExecCreateFunc != nil {
		return m.ContainerExecCreateFunc(ctx, container, config)
	}
	return types.IDResponse{ID: "mock-exec-id"}, nil
}

func (m *MockAPI) ContainerExecAttach(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
	if m.ContainerExecAttachFunc != nil {
		return m.ContainerExecAttachFunc(ctx, execID, config)
	}
	return types.HijackedResponse{}, nil
}

func (m *MockAPI) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	if m.ContainerStopFunc != nil {
		return m.ContainerStopFunc(ctx, containerID, options)
	}
	return nil
}

func (m *MockAPI) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	if m.ContainerRemoveFunc != nil {
		return m.ContainerRemoveFunc(ctx, containerID, options)
	}
	return nil
}

func (m *MockAPI) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// NewMockClient creates a new Docker client with a configurable mock API.
// It returns the Client wrapper and the underlying MockAPI struct for configuration.
func NewMockClient() (*Client, *MockAPI) {
	mock := &MockAPI{}
	return &Client{api: mock}, mock
}
