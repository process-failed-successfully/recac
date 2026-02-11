package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"recac/internal/docker"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// MockAgentForSecurity implements agent.Agent
type MockAgentForSecurity struct{}
func (m *MockAgentForSecurity) Send(ctx context.Context, prompt string) (string, error) { return "", nil }
func (m *MockAgentForSecurity) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) { return "", nil }

func TestSession_SensitiveMounts_ReadOnly(t *testing.T) {
	// Setup mock Docker client using the package-level MockDockerClient
	client, mock := docker.NewMockClient()

	var capturedBinds []string

	mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
		capturedBinds = hostConfig.Binds
		return container.CreateResponse{ID: "test-id"}, nil
	}

	// Use NewSessionWithConfig to avoid DB initialization issues
	session := NewSessionWithConfig("/tmp/workspace", "test-project", "mock", "mock-model", nil)
	session.Docker = client
	session.Agent = &MockAgentForSecurity{}
	session.Image = "alpine:latest"

	// Mock environment
	session.UseLocalAgent = false // Force Docker usage to ensure binds are processed
	mockHome := "/mock/home/user"
	session.HomeDir = mockHome
	session.StatFunc = func(path string) (os.FileInfo, error) {
		// Mock file existence for sensitive paths inside mock home
		if strings.HasPrefix(path, mockHome) {
			return nil, nil // Return nil error (file exists)
		}
		return nil, os.ErrNotExist
	}

	// Start session
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	// Verify sensitive mounts
	sensitivePaths := []string{".ssh", ".config", ".gemini", ".cursor"}

	foundAny := false
	for _, path := range sensitivePaths {
		found := false
		expectedHostPath := filepath.Join(mockHome, path)

		for _, bind := range capturedBinds {
			// Check if bind contains the sensitive path
			if strings.Contains(bind, expectedHostPath) {
				found = true
				foundAny = true
				// Check if it is Read-Only
				if !strings.HasSuffix(bind, ":ro") {
					t.Errorf("Security Vulnerability: Sensitive path '%s' is mounted Read-Write! Bind: %s", path, bind)
				}
			}
		}
		if !found {
			t.Errorf("Sensitive path '%s' was NOT mounted but should have been. Expected bind for: %s", path, expectedHostPath)
		}
	}

	if !foundAny {
		t.Error("No sensitive paths were found in binds. StatFunc mock might be failing.")
	}
}
