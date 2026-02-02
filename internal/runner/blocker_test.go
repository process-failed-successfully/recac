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

// MockDocker is a simple mock for DockerClient - unused for checking blockers now
type MockDockerForBlocker struct {
	DockerClient
}

func TestProcessResponse_BlockerFalsePositives(t *testing.T) {
	ctx := context.Background()
	// Use temp dir for workspace
	tmpDir := t.TempDir()

	mockDocker := &MockDockerForBlocker{}

	s := &Session{
		Docker:      mockDocker,
		Workspace:   tmpDir,
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
		// Write file to workspace
		filePath := filepath.Join(tmpDir, tc.filename)
		err := os.WriteFile(filePath, []byte(tc.content), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		_, err = s.ProcessResponse(ctx, "some response")

		if tc.shouldBlock {
			if err == nil || !strings.Contains(err.Error(), "blocker detected") {
				t.Errorf("Expected blocker for content '%s', but it didn't trigger", tc.content)
			}
		} else {
			if err != nil {
				t.Errorf("Did NOT expect blocker for content '%s', but it triggered: %v", tc.content, err)
			}
			// Verify file was cleaned up (removed) for false positives
			if _, err := os.Stat(filePath); !os.IsNotExist(err) {
				t.Errorf("Expected file '%s' to be deleted for false positive, but it still exists", tc.filename)
			}
		}

		// Cleanup for next test if it still exists (e.g. true positive)
		os.Remove(filePath)
	}
}
