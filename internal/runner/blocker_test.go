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
}

func (m *MockDockerForBlocker) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	return "Success", nil
}

func (m *MockDockerForBlocker) ExecAsUser(ctx context.Context, id string, user string, cmd []string) (string, error) {
	return m.Exec(ctx, id, cmd)
}

func TestProcessResponse_BlockerFalsePositives(t *testing.T) {
	ctx := context.Background()
	mockDocker := &MockDockerForBlocker{}
	workspace := t.TempDir()

	s := &Session{
		Docker:      mockDocker,
		ContainerID: "test-container",
		Workspace:   workspace,
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
		{"recac_blockers.txt", "All tests passed.", false}, // New test case for "passed"
		{"recac_blockers.txt", "Blockers: None", false}, // New test case for "Blockers: None"
	}

	for _, tc := range testCases {
		// Create file in workspace
		path := filepath.Join(workspace, tc.filename)
		os.WriteFile(path, []byte(tc.content), 0644)

		_, err := s.ProcessResponse(ctx, "some response")

		if tc.shouldBlock {
			if err == nil || !strings.Contains(err.Error(), "blocker detected") {
				t.Errorf("Expected blocker for content '%s', but it didn't trigger", tc.content)
			}
		} else {
			if err != nil {
				t.Errorf("Did NOT expect blocker for content '%s', but it triggered: %v", tc.content, err)
			}
			// Verify file was cleaned up (removed from workspace)
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("Expected file '%s' to be deleted for false positive, but it still exists", tc.filename)
			}
		}

		// Reset for next test (cleanup if not already deleted)
		os.Remove(path)
	}
}
