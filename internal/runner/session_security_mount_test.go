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
func (m *MockAgentForSecurity) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) { return "", nil }


func TestSession_SensitiveMounts_ReadOnly(t *testing.T) {
	// Setup a fake home directory
	fakeHome := "/home/testuser" // Logical path, doesn't need to exist on disk

	// Setup mock Docker client
	// Use package-level MockDockerClient to avoid CheckDaemon issues in CI
	mock := &MockDockerClient{}

	// We want to capture the binds
	var capturedBinds []string

	mock.RunContainerFunc = func(ctx context.Context, image, workspace string, extraBinds, env []string, user string) (string, error) {
		capturedBinds = extraBinds
		return "test-id", nil
	}

	// Mock ImageExists to avoid errors
	mock.ImageExistsFunc = func(ctx context.Context, image string) (bool, error) {
		return true, nil
	}

	// Mock ImageBuild to avoid errors
	mock.ImageBuildFunc = nil // Not needed if image exists

	// Use NewSessionWithConfig to avoid DB initialization issues
	session := NewSessionWithConfig("/tmp/workspace", "test-project", "mock", "mock-model", nil)
	session.Docker = mock
	session.Agent = &MockAgentForSecurity{}
	session.Image = "alpine:latest"
	session.HomeDir = fakeHome // Explicitly set HomeDir to our fake home
	session.UseLocalAgent = false // Explicitly disable Local Agent (important for CI where KUBERNETES_SERVICE_HOST is set)

	// Mock StatFunc to simulate existence of sensitive directories
	session.StatFunc = func(path string) (os.FileInfo, error) {
		base := filepath.Base(path)
		// Simulate only specific directories existing
		if base == ".ssh" || base == ".config" {
			return nil, nil // Exists (error is nil)
		}
		return nil, os.ErrNotExist
	}

	// Start session
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	// Verify sensitive mounts
	sensitivePaths := []string{".ssh", ".config"} // Only check the ones we simulated existence for

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
			t.Errorf("Expected sensitive path '%s' to be mounted, but it was not found in binds", path)
		}
	}

	// Ensure that paths we said "do not exist" (.gemini, .cursor) are NOT mounted
	unexpectedPaths := []string{".gemini", ".cursor"}
	for _, path := range unexpectedPaths {
		for _, bind := range capturedBinds {
			if strings.Contains(bind, path) {
				t.Errorf("Unexpected path '%s' was mounted even though StatFunc said it doesn't exist", path)
			}
		}
	}

	if !foundAny {
		t.Error("WARNING: No sensitive paths were found in binds.")
	}
}
