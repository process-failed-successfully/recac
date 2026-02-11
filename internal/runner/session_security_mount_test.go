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
	// Create temporary home directory
	tempHome := t.TempDir()

	// Create dummy sensitive files
	// We deliberately create .ssh, .config, .gemini.
	// We deliberately DO NOT create .cursor to verify it is NOT mounted.
	sensitivePathsToCreate := []string{".ssh", ".config", ".gemini"}

	for _, path := range sensitivePathsToCreate {
		fullPath := filepath.Join(tempHome, path)
		// Create as directory
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			t.Fatalf("Failed to create dummy dir %s: %v", fullPath, err)
		}
	}

	// Setup mock Docker client using package-level mock to avoid circular dependency
	client := &MockDockerClient{}

	// We want to capture the binds
	var capturedBinds []string

	client.RunContainerFunc = func(ctx context.Context, image, workspace string, extraBinds, env []string, user string) (string, error) {
		capturedBinds = extraBinds
		return "test-id", nil
	}

	// Mock ImageExists to return true
	client.ImageExistsFunc = func(ctx context.Context, image string) (bool, error) {
		return true, nil
	}

	// Mock ExecAsUser for git config
	client.ExecAsUserFunc = func(ctx context.Context, containerID, user string, cmd []string) (string, error) {
		return "", nil
	}

	// Use NewSessionWithConfig to avoid DB initialization issues
	session := NewSessionWithConfig("/tmp/workspace", "test-project", "mock", "mock-model", nil)
	session.Docker = client
	session.Agent = &MockAgentForSecurity{}
	session.Image = "alpine:latest"
	session.UseLocalAgent = false // Ensure we run container path
	session.HomeDir = tempHome    // Inject temp home

	// Start session
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	// Verify sensitive mounts
	// We expect .ssh, .config, .gemini to be mounted
	for _, path := range sensitivePathsToCreate {
		found := false
		for _, bind := range capturedBinds {
			if strings.Contains(bind, path) {
				found = true
				if !strings.HasSuffix(bind, ":ro") {
					t.Errorf("Security Vulnerability: Sensitive path '%s' is mounted Read-Write! Bind: %s", path, bind)
				}
                // Verify it points to our temp home
                if !strings.Contains(bind, tempHome) {
                    t.Errorf("Mount path mismatch: expected to contain %s, got %s", tempHome, bind)
                }
				break
			}
		}
		if !found {
			t.Errorf("Expected path '%s' to be mounted, but it was not found in binds", path)
		}
	}

	// Verify .cursor is NOT mounted
	for _, bind := range capturedBinds {
		if strings.Contains(bind, ".cursor") {
			t.Errorf("Unexpected mount: .cursor should not be mounted as it does not exist in temp home")
		}
	}
}
