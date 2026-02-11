package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MockAgentForSecurity implements agent.Agent
type MockAgentForSecurity struct{}

func (m *MockAgentForSecurity) Send(ctx context.Context, prompt string) (string, error) { return "", nil }
func (m *MockAgentForSecurity) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return "", nil
}

func TestSession_SensitiveMounts_ReadOnly(t *testing.T) {
	// Create a temporary home directory
	tmpHome, err := os.MkdirTemp("", "mock-home")
	if err != nil {
		t.Fatalf("Failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	// Set HOME environment variable to the temp directory
	t.Setenv("HOME", tmpHome)

	// Create dummy sensitive directories
	sensitivePaths := []string{".ssh", ".config", ".gemini", ".cursor"}
	for _, p := range sensitivePaths {
		path := filepath.Join(tmpHome, p)
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatalf("Failed to create dummy directory %s: %v", path, err)
		}
	}

	// We want to capture the binds
	var capturedBinds []string

	// Use MockDockerClient from mock_docker_test.go (same package)
	mock := &MockDockerClient{
		CheckDaemonFunc: func(ctx context.Context) error {
			return nil
		},
		ImageExistsFunc: func(ctx context.Context, image string) (bool, error) {
			return true, nil
		},
		RunContainerFunc: func(ctx context.Context, image, workspace string, extraBinds, env []string, user string) (string, error) {
			capturedBinds = extraBinds
			return "test-id", nil
		},
		ExecAsUserFunc: func(ctx context.Context, containerID, user string, cmd []string) (string, error) {
			return "", nil
		},
	}

	// Use NewSessionWithConfig to avoid DB initialization issues
	// We pass nil for dbStore
	session := NewSessionWithConfig("/tmp/workspace", "test-project", "mock", "mock-model", nil)
	session.Docker = mock
	session.Agent = &MockAgentForSecurity{}
	session.Image = "alpine:latest"

	// Start session
	// Start calls CheckDaemon, ReadSpec, ensureImage, RunContainer, fixPasswdDatabase, bootstrapGit, runInitScript
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	// Verify sensitive mounts
	foundAny := false
	for _, p := range sensitivePaths {
		expectedPath := filepath.Join(tmpHome, p)
		found := false
		for _, bind := range capturedBinds {
			// Check if bind contains the sensitive path
			if strings.Contains(bind, expectedPath) {
				found = true
				foundAny = true
				// Check if it is Read-Only
				if !strings.HasSuffix(bind, ":ro") {
					t.Errorf("Security Vulnerability: Sensitive path '%s' is mounted Read-Write! Bind: %s", p, bind)
				}
			}
		}
		if !found {
			t.Errorf("Expected sensitive path '%s' to be mounted, but it wasn't found in binds.", p)
		}
	}

	if !foundAny {
		t.Error("WARNING: No sensitive paths were found in binds even though we mocked them.")
	}
}
