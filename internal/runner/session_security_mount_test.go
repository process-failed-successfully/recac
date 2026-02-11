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
	// Create fake home dir for deterministic testing
	home := t.TempDir()

	// Create dummy sensitive directories
	sensitivePaths := []string{".ssh", ".config", ".gemini", ".cursor"}
	for _, p := range sensitivePaths {
		path := filepath.Join(home, p)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatalf("Failed to create dummy dir %s: %v", path, err)
		}
	}

	// Setup mock Docker client
	mock := &MockDockerClient{}

	// We want to capture the binds passed to RunContainer
	var capturedBinds []string

	mock.RunContainerFunc = func(ctx context.Context, image, workspace string, extraBinds, env []string, user string) (string, error) {
		capturedBinds = extraBinds
		return "test-id", nil
	}

	mock.ImageExistsFunc = func(ctx context.Context, image string) (bool, error) {
		return true, nil
	}

	mock.CheckDaemonFunc = func(ctx context.Context) error {
		return nil
	}

	mock.ExecAsUserFunc = func(ctx context.Context, containerID, user string, cmd []string) (string, error) {
		return "", nil
	}

	// Use NewSessionWithConfig to avoid DB initialization issues
	session := NewSessionWithConfig("/tmp/workspace", "test-project", "mock", "mock-model", nil)
	session.Docker = mock
	session.Agent = &MockAgentForSecurity{}
	session.Image = "alpine:latest"
	session.HomeDir = home // Set HomeDir explicitly

	// Start session
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	// Verify sensitive mounts
	foundAny := false
	for _, path := range sensitivePaths {
		found := false
		for _, bind := range capturedBinds {
			// Check if bind contains the sensitive path
			// Note: binds format is "hostPath:containerPath:ro"
			// hostPath is constructed using filepath.Join(home, path)
			expectedHostPath := filepath.Join(home, path)

			// We check if bind starts with expectedHostPath
			if strings.HasPrefix(bind, expectedHostPath) {
				found = true
				foundAny = true
				// Check if it is Read-Only
				if !strings.HasSuffix(bind, ":ro") {
					t.Errorf("Security Vulnerability: Sensitive path '%s' is mounted Read-Write! Bind: %s", path, bind)
				}
			}
		}
		if !found {
			t.Errorf("FAIL: Sensitive path '%s' not found in binds despite existing in home dir", path)
		}
	}

	if !foundAny {
		t.Error("FAIL: No sensitive paths were found in binds. Test setup failed.")
	}
}
