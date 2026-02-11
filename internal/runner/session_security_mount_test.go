package runner

import (
	"context"
	"os"
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
	// Skip if no home dir
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("Skipping test because UserHomeDir is unavailable")
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

	// Explicitly force container creation logic even in CI (where KUBERNETES_SERVICE_HOST might be set)
	session.UseLocalAgent = false

	// Start session
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	// Verify sensitive mounts with strict checking
	sensitivePaths := map[string]string{
		".ssh":    "/home/appuser/.ssh",
		".config": "/home/appuser/.config",
		".gemini": "/home/appuser/.gemini",
		".cursor": "/home/appuser/.cursor",
	}

	foundAny := false
	for name, containerPath := range sensitivePaths {
		found := false
		for _, bind := range capturedBinds {
			// Strict check: Must target the correct container path AND be Read-Only
			// Format: /host/path:/container/path:ro
			// We check suffix because host path is variable
			expectedSuffix := ":" + containerPath + ":ro"
			if strings.HasSuffix(bind, expectedSuffix) {
				found = true
				foundAny = true
				break
			}

			// Fallback check to catch missing :ro flag but correct path
			// This helps debug if the path is mounted but not RO
			pathSuffix := ":" + containerPath
			if strings.HasSuffix(bind, pathSuffix) {
				t.Errorf("Security Vulnerability: Sensitive path '%s' is mounted Read-Write! Bind: %s", name, bind)
				found = true // Mark as found to avoid "not found" log, error is already reported
				foundAny = true
				break
			}
		}
		if !found {
			t.Logf("Note: Sensitive path '%s' not found in binds (might be missing in env)", name)
		}
	}

	if !foundAny {
		t.Log("WARNING: No sensitive paths were found in binds. Test might be ineffective if environment lacks home dir configs.")
	}
}
