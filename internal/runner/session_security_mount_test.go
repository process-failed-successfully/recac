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

	// Create sensitive directories/files
	sensitiveDirs := []string{".ssh", ".config", ".gemini", ".cursor"}
	for _, dir := range sensitiveDirs {
		path := filepath.Join(tempHome, dir)
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatalf("Failed to create temp sensitive dir %s: %v", path, err)
		}
	}

	// Set HOME environment variable to tempHome
	// Note: os.UserHomeDir() respects $HOME on Unix.
	t.Setenv("HOME", tempHome)

	// Verify that os.UserHomeDir picks it up
	home, err := os.UserHomeDir()
	if err != nil || home != tempHome {
		// On some systems (like Windows or weird CI), setting HOME might not be enough.
		// However, for the purpose of this test, we assume standard Go behavior.
		// If this fails, we might need to skip or debug.
		t.Logf("Warning: os.UserHomeDir() returned '%s', expected '%s'. Test might be flaky.", home, tempHome)
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
	for _, path := range sensitiveDirs {
		found := false
		expectedHostPath := filepath.Join(tempHome, path)

		for _, bind := range capturedBinds {
			// Check if bind starts with the expected host path + ":" to ensure strict match
			if strings.HasPrefix(bind, expectedHostPath + ":") {
				found = true
				foundAny = true
				// Check if it is Read-Only
				if !strings.HasSuffix(bind, ":ro") {
					t.Errorf("Security Vulnerability: Sensitive path '%s' is mounted Read-Write! Bind: %s", path, bind)
				}
			}
		}
		if !found {
			t.Errorf("Expected sensitive path '%s' to be mounted, but it was not found in binds.", path)
		}
	}

	if !foundAny {
		t.Error("No sensitive paths were found in binds, but we created them!")
	}
}
