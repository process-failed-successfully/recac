package runner

import (
	"context"
	"recac/internal/docker"
)

type MockDockerClient struct {
	CheckDaemonFunc   func(ctx context.Context) error
	RunContainerFunc  func(ctx context.Context, image, workspace string, extraBinds, env []string, user string) (string, error)
	StopContainerFunc func(ctx context.Context, containerID string) error
	ExecFunc          func(ctx context.Context, containerID string, cmd []string) (string, error)
	ExecAsUserFunc    func(ctx context.Context, containerID, user string, cmd []string) (string, error)
	PullImageFunc     func(ctx context.Context, image string) error
	ImageExistsFunc   func(ctx context.Context, image string) (bool, error)
	ImageBuildFunc    func(ctx context.Context, options docker.ImageBuildOptions) (string, error)
}

func (m *MockDockerClient) CheckDaemon(ctx context.Context) error {
	if m.CheckDaemonFunc != nil {
		return m.CheckDaemonFunc(ctx)
	}
	return nil
}

func (m *MockDockerClient) RunContainer(ctx context.Context, image, workspace string, extraBinds, env []string, user string) (string, error) {
	if m.RunContainerFunc != nil {
		return m.RunContainerFunc(ctx, image, workspace, extraBinds, env, user)
	}
	return "mock-container-id", nil
}

func (m *MockDockerClient) StopContainer(ctx context.Context, containerID string) error {
	if m.StopContainerFunc != nil {
		return m.StopContainerFunc(ctx, containerID)
	}
	return nil
}

func (m *MockDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, containerID, cmd)
	}
	return "", nil
}

func (m *MockDockerClient) ExecAsUser(ctx context.Context, containerID, user string, cmd []string) (string, error) {
	if m.ExecAsUserFunc != nil {
		return m.ExecAsUserFunc(ctx, containerID, user, cmd)
	}
	return "", nil
}

func (m *MockDockerClient) PullImage(ctx context.Context, image string) error {
	if m.PullImageFunc != nil {
		return m.PullImageFunc(ctx, image)
	}
	return nil
}

func (m *MockDockerClient) ImageExists(ctx context.Context, image string) (bool, error) {
	if m.ImageExistsFunc != nil {
		return m.ImageExistsFunc(ctx, image)
	}
	return true, nil
}

func (m *MockDockerClient) ImageBuild(ctx context.Context, options docker.ImageBuildOptions) (string, error) {
	if m.ImageBuildFunc != nil {
		return m.ImageBuildFunc(ctx, options)
	}
	return "mock-image-id", nil
}
