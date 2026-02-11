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
	// Create a temporary home directory
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Create dummy sensitive directories
	sensitivePaths := []string{".ssh", ".config", ".gemini", ".cursor"}
	for _, p := range sensitivePaths {
		path := filepath.Join(tempHome, p)
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatalf("Failed to create sensitive dir %s: %v", path, err)
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

	// Verify sensitive mounts
	foundAny := false
	for _, path := range sensitivePaths {
		found := false
		for _, bind := range capturedBinds {
			// Check if bind contains the sensitive path (from tempHome)
			// We check for path separator to ensure we match the directory name exactly or within a path
			if strings.Contains(bind, path) {
				found = true
				foundAny = true
				// Check if it is Read-Only
				if !strings.HasSuffix(bind, ":ro") {
					t.Errorf("Security Vulnerability: Sensitive path '%s' is mounted Read-Write! Bind: %s", path, bind)
				}
			}
		}
		if !found {
			t.Errorf("Expected sensitive path '%s' to be mounted, but it was not found in binds: %v", path, capturedBinds)
		}
	}

	if !foundAny {
		t.Error("No sensitive paths were found in binds, but we created them!")
	}
}
