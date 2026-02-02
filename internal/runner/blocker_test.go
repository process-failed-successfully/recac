package runner

import (
	"context"
	"log/slog"
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
	// 1. Handle Blockers check: test -f bf && cat bf
	if len(cmd) > 2 && (strings.Contains(cmd[2], "test -f") || strings.Contains(cmd[2], "cat")) {
		// Heuristic to extract filename from "test -f <file> && cat <file>"
		// or just "cat <file>" (legacy)
		parts := strings.Fields(cmd[2])
		for _, part := range parts {
			if strings.HasSuffix(part, ".txt") {
				if content, ok := m.Files[part]; ok {
					return content, nil
				}
			}
		}
		// Return empty if file not found (simulates failing test -f or empty cat)
		return "", nil
	}

	// 2. Handle normal commands (e.g., echo) for output preservation test
	if len(cmd) > 2 {
		script := cmd[2]
		if strings.Contains(script, "echo 'Hello'") {
			return "Hello World", nil
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

func TestProcessResponse_PreservesOutputOnBlocker(t *testing.T) {
	ctx := context.Background()
	mockDocker := &MockDockerForBlocker{
		Files: map[string]string{
			"recac_blockers.txt": "I am blocked!",
		},
	}

	s := &Session{
		Docker:      mockDocker,
		ContainerID: "test-container",
		Notifier:    notify.NewManager(func(string, ...interface{}) {}),
		Logger:      slog.Default(),
	}

	// Response that generates output AND triggers a blocker check (blocker file exists)
	response := "```bash\necho 'Hello'\n```"

	output, err := s.ProcessResponse(ctx, response)

	if err != ErrBlocker {
		t.Errorf("Expected ErrBlocker, got %v", err)
	}

	if !strings.Contains(output, "Hello World") {
		t.Errorf("Expected output to be preserved, but got: '%s'", output)
	}

	if !strings.Contains(output, "[SYSTEM] BLOCKER DETECTED") {
		t.Errorf("Expected blocker message in output, but got: '%s'", output)
	}

	if !strings.Contains(output, "I am blocked!") {
		t.Errorf("Expected specific blocker content in output, but got: '%s'", output)
	}
}
