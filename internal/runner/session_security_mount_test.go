package runner

import (
	"context"
	"os"
	"path/filepath"
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
	t.Logf("MockHome: %s", mockHome)

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
		t.Logf("StatFunc called for: %s", path)
		// Only succeed for paths we expect to check, to simulate them existing
		// Use filepath.Base to be robust against path prefixes (e.g. /private/var vs /var)
		base := filepath.Base(path)
		if base == ".ssh" || base == ".config" || base == ".gemini" || base == ".cursor" {
			return nil, nil // Simulate existence
		}
		return nil, os.ErrNotExist
	}

	// Start session
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	t.Logf("Captured Binds: %v", capturedBinds)

	// Verify sensitive mounts
	sensitivePaths := []string{".ssh", ".config", ".gemini", ".cursor"}

	foundAny := false
	for _, p := range sensitivePaths {
		// Instead of matching host path prefix (which can vary by OS/environment),
		// we match by the target container path which is constant.
		// Expected bind format: HOST_PATH:/home/appuser/DIR:ro
		targetSuffix := ":/home/appuser/" + p + ":ro"

		found := false
		for _, bind := range capturedBinds {
			if strings.HasSuffix(bind, targetSuffix) {
				found = true
				foundAny = true

				// Optional: Verify host path component is reasonably correct
				// We strip the suffix to get the host path
				hostPart := strings.TrimSuffix(bind, targetSuffix)

				// Normalize both paths for comparison (handle Windows backslashes)
				expectedHostPath := filepath.Join(mockHome, p)

				// Simple check: does it look like the right path?
				// Due to symlinks (e.g. /var vs /private/var on macOS), exact string match might fail.
				// We check if the base name matches at least to ensure we are mounting the correct directory.
				if filepath.Base(hostPart) != filepath.Base(expectedHostPath) {
					t.Errorf("Bind host path mismatch for '%s'. Got: %s, Expected base: %s", p, hostPart, filepath.Base(expectedHostPath))
				}
			}
		}
		if !found {
			t.Errorf("Expected sensitive path '%s' to be mounted Read-Only to /home/appuser/%s, but it was not found in binds.", p, p)
		}
	}

	if !foundAny {
		t.Error("No sensitive paths were found in binds.")
	}
}
