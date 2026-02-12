package runner

import (
	"context"
	"fmt"
	"os"
	"recac/internal/docker"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// MockAgentForSecurity implements agent.Agent
type MockAgentForSecurity struct{}

func (m *MockAgentForSecurity) Send(ctx context.Context, prompt string) (string, error) { return "", nil }
func (m *MockAgentForSecurity) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return "", nil
}

func TestSession_SensitiveMounts_ReadOnly(t *testing.T) {
	// Use temporary directory for home to ensure absolute path on all OSes
	mockHome := t.TempDir()

	// Setup mock Docker client
	client, mock := docker.NewMockClient()

	// We want to capture the binds
	var capturedBinds []string

	mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
		capturedBinds = hostConfig.Binds
		return container.CreateResponse{ID: "test-id"}, nil
	}

	// Mock ImageList to return our image so ImageExists returns true
	mock.ImageListFunc = func(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
		return []image.Summary{{RepoTags: []string{"alpine:latest"}}}, nil
	}

	mock.PingFunc = func(ctx context.Context) (types.Ping, error) {
		return types.Ping{}, nil
	}

	mock.ContainerStartFunc = func(ctx context.Context, containerID string, options container.StartOptions) error { return nil }
	mock.ContainerExecCreateFunc = func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
		return types.IDResponse{ID: "exec-id"}, nil
	}

	// Use NewSessionWithConfig to avoid DB initialization issues
	session := NewSessionWithConfig("/tmp/workspace", "test-project", "mock", "mock-model", nil)
	session.Docker = client
	session.Agent = &MockAgentForSecurity{}
	session.Image = "alpine:latest"
	session.HomeDir = mockHome
	session.UseLocalAgent = false // Ensure we don't accidentally run locally

	// Relax StatFunc to avoid path normalization issues in CI environments (e.g. symlinks, Windows short paths)
	// We trust that Start() constructs paths correctly relative to HomeDir, so verifying binds is sufficient.
	session.StatFunc = func(path string) (os.FileInfo, error) {
		// Only succeed for paths we expect to check, to simulate them existing
		if strings.Contains(path, ".ssh") ||
		   strings.Contains(path, ".config") ||
		   strings.Contains(path, ".gemini") ||
		   strings.Contains(path, ".cursor") {
			return nil, nil // Simulate existence
		}
		return nil, os.ErrNotExist
	}

	// Start session
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	// Verify sensitive mounts
	sensitiveMounts := []struct {
		hostSuffix    string
		containerPath string
	}{
		{".gemini", "/home/appuser/.gemini"},
		{".config", "/home/appuser/.config"},
		{".cursor", "/home/appuser/.cursor"},
		{".ssh", "/home/appuser/.ssh"},
	}

	foundAny := false
	for _, m := range sensitiveMounts {
		// We check for the container path suffix and :ro flag to be robust against host path variations (e.g. symlinks, Windows)
		expectedSuffix := fmt.Sprintf(":%s:ro", m.containerPath)
		found := false
		for _, bind := range capturedBinds {
			if strings.HasSuffix(bind, expectedSuffix) {
				found = true
				foundAny = true
			}
		}
		if !found {
			t.Errorf("Expected sensitive path '%s' to be mounted as '%s', but it was not found in binds.", m.hostSuffix, expectedSuffix)
		}
	}

	if !foundAny {
		t.Logf("MockHome: %s", mockHome)
		t.Logf("Captured Binds: %v", capturedBinds)
		t.Error("No sensitive paths were found in binds.")
	}
}
