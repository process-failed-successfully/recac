package runner

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"recac/internal/notify"
	"strings"
	"testing"
)

// MockDocker is a simple mock for DockerClient
type MockDockerForBlocker struct {
	DockerClient
	Files map[string]string
}

func (m *MockDockerForBlocker) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	// Simple mock for: test -f bf && cat bf
	if len(cmd) > 2 && strings.Contains(cmd[2], "cat") {
		// Extract filename
		parts := strings.Split(cmd[2], " ")
		filename := parts[len(parts)-1]
		if content, ok := m.Files[filename]; ok {
			return content, nil
		}
	}
	// Mock for rm
	if cmd[0] == "rm" {
		delete(m.Files, cmd[1])
		return "", nil
	}
	return "", nil
}

func (m *MockDockerForBlocker) ExecAsUser(ctx context.Context, id string, user string, cmd []string) (string, error) {
	return m.Exec(ctx, id, cmd)
}

func TestProcessResponse_BlockerFalsePositives(t *testing.T) {
	ctx := context.Background()
	mockDocker := &MockDockerForBlocker{
		Files: make(map[string]string),
	}

	s := &Session{
		Docker:      mockDocker,
		ContainerID: "test-container",
		Notifier:    notify.NewManager(func(string, ...interface{}) {}),
		Logger:      slog.Default(),
	}

	testCases := []struct {
		filename    string
		content     string
		shouldBlock bool
	}{
		{"recac_blockers.txt", "No blockers identified. Initial setup complete.", false},
		{"blockers.txt", "None", false},
		{"recac_blockers.txt", "no blockers", false},
		{"recac_blockers.txt", "Initial setup complete", false},
		{"blockers.txt", "# Current Blockers\n\n# None at this time\n# The project is progressing smoothly\n# All required tools are available\n# No technical obstacles", false},
		{"recac_blockers.txt", "UI Verification Required", false},
		{"recac_blockers.txt", "I am actually blocked by missing API key", true},
		{"blockers.txt", "Error: failed to connect to DB", true},
	}

	for _, tc := range testCases {
		mockDocker.Files[tc.filename] = tc.content

		_, err := s.ProcessResponse(ctx, "some response")

		if tc.shouldBlock {
			if err == nil || !strings.Contains(err.Error(), "blocker detected") {
				t.Errorf("Expected blocker for content '%s', but it didn't trigger", tc.content)
			}
		} else {
			if err != nil {
				t.Errorf("Did NOT expect blocker for content '%s', but it triggered: %v", tc.content, err)
			}
			// Verify file was cleaned up (removed from mock map)
			if _, ok := mockDocker.Files[tc.filename]; ok {
				t.Errorf("Expected file '%s' to be deleted for false positive, but it still exists", tc.filename)
			}
		}

		// Reset for next test
		delete(mockDocker.Files, tc.filename)
	}
}

func TestProcessResponse_LocalBlocker(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	s := &Session{
		UseLocalAgent: true,
		Workspace:     tmpDir,
		Notifier:      notify.NewManager(func(string, ...interface{}) {}),
		Logger:        slog.Default(),
		// Docker is intentionally nil or not relevant for local check
	}

	// Case 1: Real Blocker
	blockerFile := filepath.Join(tmpDir, "recac_blockers.txt")
	err := os.WriteFile(blockerFile, []byte("I am truly blocked"), 0644)
	if err != nil {
		t.Fatalf("Failed to write blocker file: %v", err)
	}

	_, err = s.ProcessResponse(ctx, "some response")
	if err == nil || !strings.Contains(err.Error(), "blocker detected") {
		t.Errorf("Expected blocker detected error for local file, got: %v", err)
	}

	// Case 2: False Positive
	err = os.WriteFile(blockerFile, []byte("No blockers identified"), 0644)
	if err != nil {
		t.Fatalf("Failed to write false positive file: %v", err)
	}

	_, err = s.ProcessResponse(ctx, "some response")
	if err != nil {
		t.Errorf("Did not expect error for false positive, got: %v", err)
	}

	// Check if file was deleted
	if _, err := os.Stat(blockerFile); !os.IsNotExist(err) {
		t.Errorf("Expected false positive file to be deleted, but it exists")
	}
}
