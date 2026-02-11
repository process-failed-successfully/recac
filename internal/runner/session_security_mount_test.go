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
	session.UseLocalAgent = false // Explicitly disable Local Agent to test Docker logic

	// Inject Dependencies
	mockHome := "/mock/home/appuser"
	session.HomeDir = mockHome
	session.StatFunc = func(path string) (os.FileInfo, error) {
		// Mock existence of sensitive files
		if strings.HasPrefix(path, mockHome) {
			// Let's pretend .ssh and .config exist, but .gemini and .cursor do not
			if strings.HasSuffix(path, ".ssh") || strings.HasSuffix(path, ".config") {
				return nil, nil // Success (exists)
			}
		}
		return nil, os.ErrNotExist
	}

	// Start session
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	// Verify sensitive mounts
	// Expect: .ssh and .config
	expected := map[string]bool{
		".ssh":    true,
		".config": true,
		".gemini": false,
		".cursor": false,
	}

	for path, shouldExist := range expected {
		found := false
		for _, bind := range capturedBinds {
			if strings.Contains(bind, path) {
				found = true
				if !strings.HasSuffix(bind, ":ro") {
					t.Errorf("Security Vulnerability: Sensitive path '%s' is mounted Read-Write! Bind: %s", path, bind)
				}
			}
		}
		if shouldExist && !found {
			t.Errorf("Expected sensitive path '%s' to be mounted, but it was not.", path)
		}
		if !shouldExist && found {
			t.Errorf("Expected sensitive path '%s' NOT to be mounted (mocked as missing), but it was.", path)
		}
	}
}
