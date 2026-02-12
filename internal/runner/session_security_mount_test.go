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
	// We check for the container path because host paths might be symlinked or formatted differently in CI (e.g. t.TempDir())
	expectedMounts := []struct {
		desc          string
		containerPath string
	}{
		{".gemini", "/home/appuser/.gemini"},
		{".config", "/home/appuser/.config"},
		{".cursor", "/home/appuser/.cursor"},
		{".ssh", "/home/appuser/.ssh"},
	}

	foundCount := 0
	for _, expected := range expectedMounts {
		found := false
		for _, bind := range capturedBinds {
			// Check if bind maps to the expected container path
			// Bind format: hostPath:containerPath:ro
			targetSuffix := ":" + expected.containerPath + ":ro"
			if strings.Contains(bind, targetSuffix) || (strings.Contains(bind, ":"+expected.containerPath) && strings.HasSuffix(bind, ":ro")) {
				found = true
				foundCount++
				// Double check Read-Only just to be safe (suffix check covers it, but explicit check is good)
				if !strings.HasSuffix(bind, ":ro") {
					t.Errorf("Security Vulnerability: Sensitive path '%s' is mounted Read-Write! Bind: %s", expected.desc, bind)
				}
				break
			}
		}
		if !found {
			t.Errorf("FAIL: Sensitive path '%s' (target: %s) not found in binds despite existing in home dir", expected.desc, expected.containerPath)
		}
	}

	if foundCount == 0 {
		t.Error("FAIL: No sensitive paths were found in binds. Test setup failed.")
	}
}
