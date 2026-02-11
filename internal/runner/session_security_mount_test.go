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
	// Skip if no home dir (sanity check, though t.TempDir should work)
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

	// Mock Ping for CheckDaemon
	mock.PingFunc = func(ctx context.Context) (types.Ping, error) {
		return types.Ping{}, nil
	}

	// Use NewSessionWithConfig to avoid DB initialization issues
	session := NewSessionWithConfig("/tmp/workspace", "test-project", "mock", "mock-model", nil)
	session.Docker = client
	session.Agent = &MockAgentForSecurity{}
	session.Image = "alpine:latest"
	session.UseLocalAgent = false // Ensure we use Docker path

	// Inject Fake Home Dir using t.TempDir for cross-platform validity
	fakeHome := t.TempDir()
	session.HomeDir = fakeHome

	// Create sensitive directories so os.Stat finds them
	// This tests the real filesystem integration logic without mocking StatFunc
	sensitivePaths := []string{".ssh", ".config", ".gemini", ".cursor"}
	for _, dir := range sensitivePaths {
		path := filepath.Join(fakeHome, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatalf("Failed to create temp sensitive dir %s: %v", path, err)
		}
	}

	// Start session
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	// Verify sensitive mounts
	foundAny := false
	for _, name := range sensitivePaths {
		found := false
		expectedHostPath := filepath.Join(fakeHome, name)

		for _, bind := range capturedBinds {
			// Expected format: HOST_PATH:CONTAINER_PATH:ro
			// Check for :ro suffix
			if !strings.HasSuffix(bind, ":ro") {
				continue
			}

			// Remove suffix
			bindWithoutRO := strings.TrimSuffix(bind, ":ro")

			// Split by last colon to separate host and container path
			// This handles Windows drive letters (e.g. C:\Users\...) correctly as long as container path has no colons
			lastColon := strings.LastIndex(bindWithoutRO, ":")
			if lastColon == -1 {
				continue
			}

			hostPart := bindWithoutRO[:lastColon]
			// containerPart := bindWithoutRO[lastColon+1:]

			// Compare normalized paths
			if filepath.Clean(hostPart) == filepath.Clean(expectedHostPath) {
				found = true
				foundAny = true
			}
		}

		if !found {
			t.Errorf("Expected sensitive path '%s' to be mounted Read-Only, but it was not found correctly in binds. Expected host path: %s. Binds: %v", name, expectedHostPath, capturedBinds)
		}
	}

	if !foundAny {
		t.Error("No sensitive paths were found in binds, but they were mocked to exist.")
	}
}
