package runner

import (
	"context"
	"os"
	"strings"
	"testing"
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

	// Setup mock Docker client using the package-level MockDockerClient
	mock := &MockDockerClient{}

	// We want to capture the binds
	var capturedBinds []string

	mock.RunContainerFunc = func(ctx context.Context, image, workspace string, extraBinds, env []string, user string) (string, error) {
		capturedBinds = extraBinds
		return "test-id", nil
	}

	// Mock ImageExists to return true
	mock.ImageExistsFunc = func(ctx context.Context, image string) (bool, error) {
		return true, nil
	}

	// Mock ExecAsUser to prevent git bootstrap failure
	mock.ExecAsUserFunc = func(ctx context.Context, containerID, user string, cmd []string) (string, error) {
		return "", nil
	}

	// Use NewSessionWithConfig to avoid DB initialization issues
	session := NewSessionWithConfig("/tmp/workspace", "test-project", "mock", "mock-model", nil)
	session.Docker = mock
	session.Agent = &MockAgentForSecurity{}
	session.Image = "alpine:latest"

	// Start session
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	// Verify sensitive mounts
	sensitivePaths := []string{".ssh", ".config", ".gemini", ".cursor"}

	foundAny := false
	for _, path := range sensitivePaths {
		found := false
		for _, bind := range capturedBinds {
			// Check if bind contains the sensitive path
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
			t.Logf("Note: Sensitive path '%s' not found in binds (might be missing in env)", path)
		}
	}

	if !foundAny {
		t.Log("WARNING: No sensitive paths were found in binds. Test might be ineffective if environment lacks home dir configs.")
	}
}
