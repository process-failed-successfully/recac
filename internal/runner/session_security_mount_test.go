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
	// Setup mock Docker client using internal package mock to avoid circular deps
	mock := &MockDockerClient{}

	// We want to capture the binds
	var capturedBinds []string
	var runContainerCalled bool

	mock.CheckDaemonFunc = func(ctx context.Context) error {
		return nil
	}

	mock.RunContainerFunc = func(ctx context.Context, image, workspace string, extraBinds, env []string, user string) (string, error) {
		runContainerCalled = true
		capturedBinds = extraBinds
		return "test-id", nil
	}

	// Mock ImageExists so ensureImage passes
	mock.ImageExistsFunc = func(ctx context.Context, image string) (bool, error) {
		return true, nil
	}

	// Mock ExecAsUser for git config (bootstrapGit)
	mock.ExecAsUserFunc = func(ctx context.Context, containerID, user string, cmd []string) (string, error) {
		return "mock-exec-output", nil
	}

	// Use NewSessionWithConfig to avoid DB initialization issues
	session := NewSessionWithConfig("/tmp/workspace", "test-project", "mock", "mock-model", nil)
	session.Docker = mock
	session.Agent = &MockAgentForSecurity{}
	session.Image = "alpine:latest"
	// Explicitly disable local agent mode to ensure Docker path is taken
	session.UseLocalAgent = false

	// Configure mock filesystem environment to ensure deterministic test behavior
	mockHome := "/mock/home"
	session.HomeDir = mockHome
	session.StatFunc = func(path string) (os.FileInfo, error) {
		// Mock existence for our sensitive paths
		// Normalize paths to forward slashes for robust comparison (handles Windows paths)
		normalizedPath := filepath.ToSlash(path)
		normalizedMockHome := filepath.ToSlash(mockHome)

		// Paths will be like /mock/home/.ssh
		if strings.HasPrefix(normalizedPath, normalizedMockHome) {
			return nil, nil // Return nil info and nil error (success)
		}
		return nil, os.ErrNotExist
	}

	// Start session
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	if !runContainerCalled {
		t.Fatal("RunContainer was not called! Test is ineffective.")
	}

	// Verify sensitive mounts
	sensitivePaths := []string{".ssh", ".config", ".gemini", ".cursor"}

	foundAny := false
	for _, path := range sensitivePaths {
		expectedPath := filepath.Join(mockHome, path)
		found := false
		for _, bind := range capturedBinds {
			// Bind format: /host/path:/container/path:ro
			// Check if bind starts with the expected host path
			if strings.HasPrefix(bind, expectedPath) {
				found = true
				foundAny = true
				// Check if it is Read-Only
				if !strings.HasSuffix(bind, ":ro") {
					t.Errorf("Security Vulnerability: Sensitive path '%s' is mounted Read-Write! Bind: %s", path, bind)
				}
			}
		}
		if !found {
			t.Errorf("Expected sensitive path '%s' (host: %s) to be mounted, but it was not found in binds: %v", path, expectedPath, capturedBinds)
		}
	}

	if !foundAny {
		t.Errorf("No sensitive paths were found in binds. Test failed to simulate sensitive files.")
	}
}
