package runner

import (
	"context"
	"recac/internal/docker"
)

// MockDockerClient for testing
type MockDockerClient struct {
	ExecFunc func(ctx context.Context, containerID string, cmd []string) (string, error)
}

func (m *MockDockerClient) CheckDaemon(ctx context.Context) error { return nil }
func (m *MockDockerClient) RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error) {
	return "mock-id", nil
}
func (m *MockDockerClient) StopContainer(ctx context.Context, containerID string) error { return nil }
func (m *MockDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, containerID, cmd)
	}
	return "", nil
}
func (m *MockDockerClient) ExecAsUser(ctx context.Context, containerID string, user string, cmd []string) (string, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, containerID, cmd)
	}
	return "", nil
}
func (m *MockDockerClient) ImageExists(ctx context.Context, tag string) (bool, error) {
	return true, nil
}
func (m *MockDockerClient) ImageBuild(ctx context.Context, opts docker.ImageBuildOptions) (string, error) {
	return opts.Tag, nil
}
func (m *MockDockerClient) PullImage(ctx context.Context, imageRef string) error { return nil }
