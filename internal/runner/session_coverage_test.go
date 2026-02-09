package runner

import (
	"context"
	"errors"
	"recac/internal/docker"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// CoverageMockDockerClient implements DockerClient interface for testing
type CoverageMockDockerClient struct {
	CheckDaemonFunc   func(ctx context.Context) error
	RunContainerFunc  func(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error)
	StopContainerFunc func(ctx context.Context, containerID string) error
	ExecFunc          func(ctx context.Context, containerID string, cmd []string) (string, error)
	ExecAsUserFunc    func(ctx context.Context, containerID string, user string, cmd []string) (string, error)
	ImageExistsFunc   func(ctx context.Context, tag string) (bool, error)
	ImageBuildFunc    func(ctx context.Context, opts docker.ImageBuildOptions) (string, error)
	PullImageFunc     func(ctx context.Context, imageRef string) error
}

func (m *CoverageMockDockerClient) CheckDaemon(ctx context.Context) error {
	if m.CheckDaemonFunc != nil { return m.CheckDaemonFunc(ctx) }
	return nil
}
func (m *CoverageMockDockerClient) RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error) {
	if m.RunContainerFunc != nil { return m.RunContainerFunc(ctx, imageRef, workspace, extraBinds, env, user) }
	return "mock-container-id", nil
}
func (m *CoverageMockDockerClient) StopContainer(ctx context.Context, containerID string) error {
	if m.StopContainerFunc != nil { return m.StopContainerFunc(ctx, containerID) }
	return nil
}
func (m *CoverageMockDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	if m.ExecFunc != nil { return m.ExecFunc(ctx, containerID, cmd) }
	return "", nil
}
func (m *CoverageMockDockerClient) ExecAsUser(ctx context.Context, containerID string, user string, cmd []string) (string, error) {
	if m.ExecAsUserFunc != nil { return m.ExecAsUserFunc(ctx, containerID, user, cmd) }
	return "", nil
}
func (m *CoverageMockDockerClient) ImageExists(ctx context.Context, tag string) (bool, error) {
	if m.ImageExistsFunc != nil { return m.ImageExistsFunc(ctx, tag) }
	return true, nil
}
func (m *CoverageMockDockerClient) ImageBuild(ctx context.Context, opts docker.ImageBuildOptions) (string, error) {
	if m.ImageBuildFunc != nil { return m.ImageBuildFunc(ctx, opts) }
	return "mock-image-id", nil
}
func (m *CoverageMockDockerClient) PullImage(ctx context.Context, imageRef string) error {
	if m.PullImageFunc != nil { return m.PullImageFunc(ctx, imageRef) }
	return nil
}

func TestSession_ProcessResponse_Timeout_Coverage(t *testing.T) {
	viper.Set("bash_timeout", 1) // 1 second timeout
	defer viper.Set("bash_timeout", 600)

	mockDocker := &CoverageMockDockerClient{}
	mockDocker.ExecFunc = func(ctx context.Context, containerID string, cmd []string) (string, error) {
		// Identify the sleep command
		if len(cmd) > 2 && strings.Contains(cmd[2], "sleep 2") {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(2 * time.Second): // Simulate long running
				return "done", nil
			}
		}
		// Blocker check or other commands -> return empty (no blocker)
		return "", nil
	}

	session := NewSession(mockDocker, &MockAgent{}, "/tmp", "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.ContainerID = "test-container"

	response := "```bash\nsleep 2\n```"
	output, err := session.ProcessResponse(context.Background(), response)

	if err != nil {
		t.Logf("Got expected error: %v", err)
	}
	// ProcessResponse catches the error and logs it to output, breaking loop.
	// It checks for DeadlineExceeded and formats message.
	if !strings.Contains(output, "Command timed out") {
		t.Errorf("Expected timeout message in output, got: %s", output)
	}
}

func TestSession_ProcessResponse_JSONBlock_Coverage(t *testing.T) {
	session := NewSession(nil, &MockAgent{}, "/tmp", "alpine", "test-project", "gemini", "gemini-pro", 1)

	response := "```bash\n{\"key\": \"value\"}\n```"
	output, _ := session.ProcessResponse(context.Background(), response)

	if !strings.Contains(output, "Skipped JSON Block") {
		t.Errorf("Expected JSON block skip message, got: %s", output)
	}
}

func TestSession_BootstrapGit_Error_Coverage(t *testing.T) {
	mockDocker := &CoverageMockDockerClient{}
	mockDocker.ExecAsUserFunc = func(ctx context.Context, containerID, user string, cmd []string) (string, error) {
		return "", errors.New("exec error")
	}

	session := NewSession(mockDocker, &MockAgent{}, "/tmp", "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.ContainerID = "test-container"

	err := session.bootstrapGit(context.Background())
	if err == nil {
		t.Error("Expected error from bootstrapGit, got nil")
	}
}

func TestSession_FixPermissions_Error_Coverage(t *testing.T) {
	mockDocker := &CoverageMockDockerClient{}
	mockDocker.ExecAsUserFunc = func(ctx context.Context, containerID, user string, cmd []string) (string, error) {
		return "", errors.New("chown error")
	}

	session := NewSession(mockDocker, &MockAgent{}, "/tmp", "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.ContainerID = "test-container"

	err := session.fixPermissions(context.Background())
	if err == nil {
		t.Error("Expected error from fixPermissions, got nil")
	}
}
