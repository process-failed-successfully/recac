package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"recac/internal/docker"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// MockAgentForSecurity implements agent.Agent
type MockAgentForSecurity struct{}
func (m *MockAgentForSecurity) Send(ctx context.Context, prompt string) (string, error) { return "", nil }
func (m *MockAgentForSecurity) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) { return "", nil }


func TestSession_SensitiveMounts_ReadOnly(t *testing.T) {
	// Create a temporary directory to act as HOME
	tempHome := t.TempDir()

	// Set HOME environment variable to the temporary directory
	t.Setenv("HOME", tempHome)

	// Create dummy sensitive directories
	// We intentionally create only some of them to verify conditional logic
	sensitiveDirs := []string{".ssh", ".config", ".gemini"}
	for _, dir := range sensitiveDirs {
		path := filepath.Join(tempHome, dir)
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatalf("Failed to create dummy directory %s: %v", path, err)
		}
	}

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

	mock.ContainerStartFunc = func(ctx context.Context, containerID string, options container.StartOptions) error { return nil }
	mock.ContainerExecCreateFunc = func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
		return types.IDResponse{ID: "exec-id"}, nil
	}

	// Use NewSessionWithConfig to avoid DB initialization issues
	session := NewSessionWithConfig("/tmp/workspace", "test-project", "mock", "mock-model", nil)
	session.Docker = client
	session.Agent = &MockAgentForSecurity{}
	session.Image = "alpine:latest"

	// Start session
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	// Verify sensitive mounts are present and read-only
	for _, dir := range sensitiveDirs {
		found := false
		expectedPath := filepath.Join(tempHome, dir)
		for _, bind := range capturedBinds {
			if strings.Contains(bind, expectedPath) {
				found = true
				if !strings.HasSuffix(bind, ":ro") {
					t.Errorf("Security Vulnerability: Sensitive path '%s' is mounted Read-Write! Bind: %s", dir, bind)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected sensitive path '%s' to be mounted, but it was not found in binds: %v", dir, capturedBinds)
		}
	}

	// Verify that non-existent directories are NOT mounted
	// We didn't create .cursor, so it should not be mounted
	notCreated := ".cursor"
	unexpectedPath := filepath.Join(tempHome, notCreated)
	for _, bind := range capturedBinds {
		if strings.Contains(bind, unexpectedPath) {
			t.Errorf("Unexpected mount found for non-existent directory '%s': %s", notCreated, bind)
		}
	}
}
