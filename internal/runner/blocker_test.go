package runner

import (
	"context"
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

func TestProcessResponse_BlockerFalsePositives(t *testing.T) {
	ctx := context.Background()
	mockDocker := &MockDockerForBlocker{
		Files: make(map[string]string),
	}

	s := &Session{
		Docker:      mockDocker,
		ContainerID: "test-container",
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
		{"recac_blockers.txt", "UI Verification Required", true},
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
