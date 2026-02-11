package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// MockAgentForSecurity implements agent.Agent
type MockAgentForSecurity struct{}

func (m *MockAgentForSecurity) Send(ctx context.Context, prompt string) (string, error) { return "", nil }
func (m *MockAgentForSecurity) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return "", nil
}

func TestSession_SensitiveMounts_ReadOnly(t *testing.T) {
	// Setup mock Docker client using local MockDockerClient
	mock := &MockDockerClient{}
	var capturedBinds []string

	// We only need to mock RunContainer and ImageExists for this test
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

	// Use NewSessionWithConfig to avoid DB initialization issues
	session := NewSessionWithConfig("/tmp/workspace", "test-project", "mock", "mock-model", nil)
	session.Docker = mock
	session.Agent = &MockAgentForSecurity{}
	session.Image = "alpine:latest"

	// Mock HomeDir to ensure consistent path testing
	session.HomeDir = "/home/mockuser"

	// Mock StatFunc to simulate sensitive files existence
	session.StatFunc = func(path string) (os.FileInfo, error) {
		// Simulate that all sensitive paths exist
		// Paths are constructed using filepath.Join(session.HomeDir, ...)
		if strings.HasPrefix(path, filepath.Join(session.HomeDir, ".")) {
			return &mockFileInfo{name: filepath.Base(path)}, nil
		}

		// Also simulate workspace Dockerfile missing (normal case)
		if strings.Contains(path, "Dockerfile") {
			return nil, os.ErrNotExist
		}
		// Simulate feature_list.json missing (normal case)
		if strings.Contains(path, "feature_list.json") {
			return nil, os.ErrNotExist
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
			t.Errorf("Expected sensitive path '%s' to be mounted, but it was not found in binds: %v", path, capturedBinds)
		}
	}

	if !foundAny {
		t.Error("No sensitive paths were found in binds, even though StatFunc reported them as existing.")
	}
}

// mockFileInfo implements os.FileInfo for testing
type mockFileInfo struct {
	name string
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return 0 }
func (m *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return false }
func (m *mockFileInfo) Sys() interface{}   { return nil }
