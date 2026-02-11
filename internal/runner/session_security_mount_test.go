package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"recac/internal/agent"
)

func TestSession_SensitiveMounts_ReadOnly(t *testing.T) {
	// Create a temporary directory for home
	tempHome, err := os.MkdirTemp("", "recac-test-home")
	if err != nil {
		t.Fatalf("Failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tempHome)

	// Create dummy sensitive files/directories
	sensitivePaths := []string{".gemini", ".config", ".cursor", ".ssh"}
	for _, p := range sensitivePaths {
		path := filepath.Join(tempHome, p)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatalf("Failed to create sensitive path %s: %v", path, err)
		}
	}

	// Capture binds
	var capturedBinds []string

	// Setup mock Docker client using the package-level struct
	mockDocker := &MockDockerClient{
		RunContainerFunc: func(ctx context.Context, image, workspace string, extraBinds, env []string, user string) (string, error) {
			capturedBinds = extraBinds
			return "test-container-id", nil
		},
		ImageExistsFunc: func(ctx context.Context, image string) (bool, error) {
			return true, nil
		},
		ExecAsUserFunc: func(ctx context.Context, containerID, user string, cmd []string) (string, error) {
			return "", nil // Success
		},
	}

	// Use NewSessionWithConfig to initialize basic session
	session := NewSessionWithConfig("/tmp/workspace", "test-project", "mock", "mock-model", nil)

	// Inject dependencies
	session.Docker = mockDocker
	session.Agent = agent.NewMockAgent()
	session.Image = "alpine:latest"
	session.HomeDir = tempHome
	session.UseLocalAgent = false // Ensure we test Docker path

	// Start session
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	// Verify sensitive mounts
	for _, path := range sensitivePaths {
		found := false

		// Expected bind format: /path/to/home/.ssh:/home/appuser/.ssh:ro
		// We need to check if the host path matches our tempHome joined with the sensitive path
		expectedHostPath := filepath.Join(tempHome, path)

		for _, bind := range capturedBinds {
			parts := strings.Split(bind, ":")
			if len(parts) >= 2 {
				if parts[0] == expectedHostPath {
					found = true

					// Check for Read-Only flag
					isRO := false
					if len(parts) >= 3 && parts[2] == "ro" {
						isRO = true
					}

					if !isRO {
						t.Errorf("Security Vulnerability: Sensitive path '%s' is mounted Read-Write! Bind: %s", path, bind)
					}
				}
			}
		}

		if !found {
			t.Errorf("Sensitive path '%s' not found in binds. Captured binds: %v", path, capturedBinds)
		}
	}
}
