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
	mockHome := t.TempDir()

	// Create sensitive directories/files in mockHome
	sensitivePaths := []string{".ssh", ".config", ".gemini", ".cursor"}
	for _, path := range sensitivePaths {
		fullPath := filepath.Join(mockHome, path)
		if err := os.MkdirAll(fullPath, 0700); err != nil {
			t.Fatalf("Failed to create mock sensitive dir %s: %v", fullPath, err)
		}
		// Create a dummy file inside to simulate content
		if err := os.WriteFile(filepath.Join(fullPath, "dummy"), []byte("secret"), 0600); err != nil {
			t.Fatalf("Failed to create dummy file in %s: %v", fullPath, err)
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

	// Use NewSessionWithConfig to avoid DB initialization issues and to ensure UseLocalAgent is false
	// We create a mock workspace as well
	workspace := filepath.Join(t.TempDir(), "workspace")
	os.MkdirAll(workspace, 0755)

	session := NewSessionWithConfig(workspace, "test-project", "mock", "mock-model", nil)
	session.Docker = client
	session.Agent = &MockAgentForSecurity{}
	session.Image = "alpine:latest"
	session.HomeDir = mockHome // Explicitly inject mock home directory

	// Start session
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	// Verify sensitive mounts
	foundCount := 0
	for _, path := range sensitivePaths {
		found := false
		for _, bind := range capturedBinds {
			// Check if bind contains the sensitive path
			// Bind format: /host/path:/container/path:ro
			// We check if the host path matches our mock home path
			expectedHostPath := filepath.Join(mockHome, path)

			if strings.HasPrefix(bind, expectedHostPath) {
				found = true
				foundCount++

				// Check if it is Read-Only
				if !strings.HasSuffix(bind, ":ro") {
					t.Errorf("Security Vulnerability: Sensitive path '%s' is mounted Read-Write! Bind: %s", path, bind)
				} else {
					t.Logf("Verified Read-Only mount: %s", bind)
				}
			}
		}
		if !found {
			t.Errorf("Expected sensitive path '%s' to be mounted, but it was not found in binds.", path)
		}
	}

	if foundCount == 0 {
		t.Fatal("No sensitive paths were found in binds. Test setup failed.")
	}
}
