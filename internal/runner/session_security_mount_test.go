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
	// Create a temporary directory to act as HOME
	home := t.TempDir()

	// Create sensitive directories
	sensitivePaths := []string{".ssh", ".config", ".gemini", ".cursor"}
	for _, path := range sensitivePaths {
		err := os.MkdirAll(filepath.Join(home, path), 0755)
		if err != nil {
			t.Fatalf("Failed to create dummy sensitive directory %s: %v", path, err)
		}
	}

	// We want to capture the binds
	var capturedBinds []string

	// Setup MockDockerClient (bypassing internal/docker logic entirely)
	// We use the MockDockerClient defined in mock_docker_test.go within the same package
	mockDocker := &MockDockerClient{}
	mockDocker.CheckDaemonFunc = func(ctx context.Context) error {
		return nil
	}
	mockDocker.RunContainerFunc = func(ctx context.Context, image, workspace string, extraBinds, env []string, user string) (string, error) {
		capturedBinds = extraBinds
		return "test-id", nil
	}
	mockDocker.ImageExistsFunc = func(ctx context.Context, image string) (bool, error) {
		return true, nil // Pretend image exists
	}
	// Needed for bootstrapGit or others if called
	mockDocker.ExecAsUserFunc = func(ctx context.Context, containerID, user string, cmd []string) (string, error) {
		return "", nil
	}
	// Needed for runInitScript check (chmod)
	mockDocker.ExecFunc = func(ctx context.Context, containerID string, cmd []string) (string, error) {
		return "", nil
	}


	// Use NewSessionWithConfig to avoid DB initialization issues
	session := NewSessionWithConfig("/tmp/workspace", "test-project", "mock", "mock-model", nil)
	session.HomeDir = home
	session.Docker = mockDocker
	session.Agent = &MockAgentForSecurity{}
	session.Image = "alpine:latest"
	session.UseLocalAgent = false // Explicitly disable local agent to ensure container creation logic runs

	// Start session
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	if capturedBinds == nil {
		t.Fatal("FATAL: Container was not created (capturedBinds is nil). This likely means UseLocalAgent was true or Docker check failed.")
	}

	foundAny := false
	for _, path := range sensitivePaths {
		found := false
		for _, bind := range capturedBinds {
			// Check if bind contains the sensitive path
			// Note: bind format is /host/path:/container/path:ro
			// Since we use a temp dir, the host path will start with the temp dir path
			expectedHostPath := filepath.Join(home, path)
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
			t.Logf("Note: Sensitive path '%s' not found in binds (might be missing in env)", path)
		}
	}

	if !foundAny {
		t.Fatal("FATAL: No sensitive paths were found in binds even though they exist in HOME.")
	}
}
