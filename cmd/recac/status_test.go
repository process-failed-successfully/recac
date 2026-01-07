package main

import (
	"bytes"
	"context"
	"errors"
	"recac/internal/docker"
	"recac/internal/runner"
	"recac/internal/ui"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/spf13/viper"
)

// MockSessionManager is a mock implementation of the SessionManager for testing.
type MockSessionManager struct {
	Sessions []*runner.SessionState
	Error    error
}

func (m *MockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.Sessions, nil
}

func (m *MockSessionManager) LoadSession(name string) (*runner.SessionState, error) {
	return nil, nil // Not needed for this test
}

func (m *MockSessionManager) GetSessionLogs(name string) (string, error) {
	return "", nil // Not needed for this test
}

func (m *MockSessionManager) StartSession(name string, command []string, workspace string) (*runner.SessionState, error) {
	return nil, nil // Not needed for this test
}

func (m *MockSessionManager) StopSession(name string) error {
	return nil // Not needed for this test
}

// MockDockerClient is a mock implementation of the Docker client for testing.
type MockDockerClient struct {
	Version types.Version
	Err     error
}

func (m *MockDockerClient) ServerVersion(ctx context.Context) (types.Version, error) {
	if m.Err != nil {
		return types.Version{}, m.Err
	}
	return m.Version, nil
}

func (m *MockDockerClient) Close() error {
	return nil
}

func (m *MockDockerClient) CheckDaemon(ctx context.Context) error {
	return nil
}

func (m *MockDockerClient) CheckSocket(ctx context.Context) error {
	return nil
}

func (m *MockDockerClient) CheckImage(ctx context.Context, imageRef string) (bool, error) {
	return true, nil
}

func (m *MockDockerClient) PullImage(ctx context.Context, imageRef string) error {
	return nil
}

func (m *MockDockerClient) ImageBuild(ctx context.Context, opts docker.ImageBuildOptions) (string, error) {
	return "", nil
}

func (m *MockDockerClient) RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error) {
	return "", nil
}

func (m *MockDockerClient) StopContainer(ctx context.Context, containerID string) error {
	return nil
}

func (m *MockDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	return "", nil
}

func (m *MockDockerClient) ExecAsUser(ctx context.Context, containerID string, user string, cmd []string) (string, error) {
	return "", nil
}

func (m *MockDockerClient) ImageExists(ctx context.Context, tag string) (bool, error) {
	return true, nil
}

func TestStatusCmd(t *testing.T) {
	// --- Setup ---
	// Keep original functions to restore them later
	originalNewSessionManager := ui.NewSessionManager
	originalNewDockerClient := ui.NewDockerClient
	defer func() {
		ui.NewSessionManager = originalNewSessionManager
		ui.NewDockerClient = originalNewDockerClient
		viper.Reset()
	}()

	// --- Mock Implementations ---
	ui.NewSessionManager = func() (runner.ISessionManager, error) {
		return &MockSessionManager{
			Sessions: []*runner.SessionState{
				{
					Name:      "test-session-1",
					PID:       1234,
					Status:    "RUNNING",
					StartTime: time.Now(),
					Workspace: "/tmp/workspace1",
				},
			},
		}, nil
	}

	ui.NewDockerClient = func(projectName string) (docker.IClient, error) {
		return &MockDockerClient{
			Version: types.Version{
				Version:    "20.10.7",
				APIVersion: "1.41",
				Os:         "linux",
				Arch:       "amd64",
			},
		}, nil
	}

	// Set mock config values
	viper.Set("provider", "test-provider")
	viper.Set("model", "test-model")

	// --- Test Execution ---
	// Redirect Cobra command output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"status"})
	err := rootCmd.Execute()

	// --- Assertions ---
	if err != nil {
		t.Fatalf("status command failed: %v", err)
	}

	output := buf.String()

	// Check for all sections
	if !strings.Contains(output, "--- RECAC Status ---") {
		t.Error("output should contain '--- RECAC Status ---'")
	}
	if !strings.Contains(output, "[Sessions]") {
		t.Error("output should contain '[Sessions]' section")
	}
	if !strings.Contains(output, "[Docker Environment]") {
		t.Error("output should contain '[Docker Environment]' section")
	}
	if !strings.Contains(output, "[Configuration]") {
		t.Error("output should contain '[Configuration]' section")
	}

	// Check for specific values
	if !strings.Contains(output, "test-session-1") {
		t.Error("output should contain session name 'test-session-1'")
	}
	if !strings.Contains(output, "Docker Version: 20.10.7") {
		t.Error("output should contain 'Docker Version: 20.10.7'")
	}
	if !strings.Contains(output, "Provider: test-provider") {
		t.Error("output should contain 'Provider: test-provider'")
	}
	if !strings.Contains(output, "Model: test-model") {
		t.Error("output should contain 'Model: test-model'")
	}

	// --- Test Error Case: Docker Down ---
	ui.NewDockerClient = func(projectName string) (docker.IClient, error) {
		return &MockDockerClient{
			Err: errors.New("docker daemon is not running"),
		}, nil
	}

	buf.Reset()
	rootCmd.SetArgs([]string{"status"})
	err = rootCmd.Execute()

	if err != nil {
		t.Fatalf("status command failed on docker error case: %v", err)
	}

	output = buf.String()

	if !strings.Contains(output, "Could not connect to Docker daemon") {
		t.Error("output should show a docker connection error")
	}

}
