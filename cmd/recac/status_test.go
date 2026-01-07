package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"recac/internal/docker"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDockerClient is a mock implementation of the Docker API client
type mockDockerClient struct {
	docker.APIClient
	serverVersion types.Version
	serverError   error
}

func (m *mockDockerClient) ServerVersion(ctx context.Context) (types.Version, error) {
	return m.serverVersion, m.serverError
}

func (m *mockDockerClient) Ping(ctx context.Context) (types.Ping, error) {
	return types.Ping{}, nil
}

func (m *mockDockerClient) ImageList(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
	return nil, nil
}

func (m *mockDockerClient) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (m *mockDockerClient) ImageBuild(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (types.ImageBuildResponse, error) {
	return types.ImageBuildResponse{Body: io.NopCloser(bytes.NewReader(nil))}, nil
}

func (m *mockDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
	return container.CreateResponse{}, nil
}

func (m *mockDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	return nil
}

func (m *mockDockerClient) ContainerExecCreate(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
	return types.IDResponse{}, nil
}

func (m *mockDockerClient) ContainerExecAttach(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
	return types.HijackedResponse{}, nil
}

func (m *mockDockerClient) ContainerExecInspect(ctx context.Context, execID string) (container.ExecInspect, error) {
	return container.ExecInspect{}, nil
}

func (m *mockDockerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	return nil
}

func (m *mockDockerClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	return nil
}

func (m *mockDockerClient) Close() error {
	return nil
}

// overrideDependencies is a helper to override dependencies for testing
func overrideDependencies(sm *runner.SessionManager, dockerClient docker.APIClient) func() {
	originalNewSessionManager := newSessionManager
	originalNewDockerClient := newDockerClient

	newSessionManager = func() (*runner.SessionManager, error) {
		return sm, nil
	}
	newDockerClient = func(string) (docker.APIClient, error) {
		return dockerClient, nil
	}

	return func() {
		newSessionManager = originalNewSessionManager
		newDockerClient = originalNewDockerClient
	}
}

// captureOutput executes a function and captures its standard output.
func captureOutput(f func() error) (string, error) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String(), err
}

func TestShowStatus_WithSessions(t *testing.T) {
	// 1. Setup
	tempDir := t.TempDir()
	sm, err := runner.NewSessionManagerWithDir(filepath.Join(tempDir, "sessions"))
	require.NoError(t, err)

	mockDocker := &mockDockerClient{
		serverVersion: types.Version{
			Version:    "20.10.7",
			APIVersion: "1.41",
			Os:         "linux",
			Arch:       "amd64",
		},
	}

	// Override dependencies
	cleanup := overrideDependencies(sm, mockDocker)
	defer cleanup()

	// Create a fake session
	fakeSession := &runner.SessionState{
		Name:      "test-session-123",
		PID:       os.Getpid(),
		StartTime: time.Date(2023, 1, 1, 10, 30, 0, 0, time.UTC),
		Status:    "running",
		Workspace: "/tmp/workspace1",
	}
	err = sm.SaveSession(fakeSession)
	require.NoError(t, err)

	viper.Set("provider", "test-provider")
	viper.Set("model", "test-model")
	viper.Set("config", "/tmp/config.yaml")
	defer viper.Reset()

	// 2. Execute
	output, err := captureOutput(showStatus)
	require.NoError(t, err)

	// 3. Assert
	// Assert Session Output
	assert.Contains(t, output, "[Sessions]", "Output should contain session header")
	assert.Contains(t, output, "NAME", "Output should contain table headers")
	assert.Contains(t, output, "test-session-123", "Output should contain session name")
	assert.Contains(t, output, "running", "Output should contain session status")
	assert.Contains(t, output, fmt.Sprintf("%d", fakeSession.PID), "Output should contain session PID")
	assert.Contains(t, output, "2023-01-01 10:30:00", "Output should contain formatted start time")
	assert.Contains(t, output, "/tmp/workspace1", "Output should contain session workspace")

	// Assert Docker Output
	assert.Contains(t, output, "[Docker Environment]", "Output should contain Docker header")
	assert.Contains(t, output, "Docker Version: 20.10.7", "Output should contain Docker version")

	// Assert Configuration Output
	assert.Contains(t, output, "[Configuration]", "Output should contain Configuration header")
	assert.Contains(t, output, "Provider: test-provider", "Output should contain provider")
}

func TestShowStatus_NoSessions(t *testing.T) {
	// 1. Setup
	tempDir := t.TempDir()
	sm, err := runner.NewSessionManagerWithDir(filepath.Join(tempDir, "sessions"))
	require.NoError(t, err)

	mockDocker := &mockDockerClient{} // Default empty mock
	cleanup := overrideDependencies(sm, mockDocker)
	defer cleanup()

	// 2. Execute
	output, err := captureOutput(showStatus)
	require.NoError(t, err)

	// 3. Assert
	assert.Contains(t, output, "No sessions found.", "Output should indicate no sessions were found")
}

func TestShowStatus_DockerError(t *testing.T) {
	// 1. Setup
	tempDir := t.TempDir()
	sm, err := runner.NewSessionManagerWithDir(filepath.Join(tempDir, "sessions"))
	require.NoError(t, err)

	mockDocker := &mockDockerClient{
		serverError: assert.AnError,
	}
	cleanup := overrideDependencies(sm, mockDocker)
	defer cleanup()

	// 2. Execute
	output, err := captureOutput(showStatus)
	require.NoError(t, err)

	// 3. Assert
	assert.Contains(t, output, "Could not connect to Docker daemon.", "Output should show a Docker connection error")
}
