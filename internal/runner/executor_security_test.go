package runner

import (
	"context"
	"log/slog"
	"recac/internal/notify"
	"recac/internal/security"
	"strings"
	"testing"
)

// SecurityMockDocker is a local mock for testing security features
// independent of agent_exec_test.go's MockDockerForExec
type SecurityMockDocker struct {
	DockerClient
	ExecutedCmds []string
}

func (m *SecurityMockDocker) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	fullCmd := strings.Join(cmd, " ")
	m.ExecutedCmds = append(m.ExecutedCmds, fullCmd)
	// Simulate empty blocker file to avoid triggering blocker detection
	if strings.Contains(fullCmd, "cat recac_blockers.txt") || strings.Contains(fullCmd, "cat blockers.txt") {
		return "", nil
	}
	return "Success: " + fullCmd, nil
}

func TestProcessResponse_Security(t *testing.T) {
	mockDocker := &SecurityMockDocker{}
	s := &Session{
		Docker:      mockDocker,
		ContainerID: "test-container",
		Notifier:    notify.NewManager(func(string, ...interface{}) {}),
		Logger:      slog.Default(),
		Scanner:     security.NewRegexScanner(), // Use real scanner
	}

	// 1. Dangerous Command
	resp := "I will delete everything.\n```bash\nrm -rf /\n```"
	out, err := s.ProcessResponse(context.Background(), resp)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	// Verify it was blocked
	if !strings.Contains(out, "[BLOCKED] Command 1 blocked by security scanner") {
		t.Errorf("Expected blocked message, got: %s", out)
	}
	if !strings.Contains(out, "Root Deletion") { // Description from scanner
		t.Errorf("Expected description 'Root Deletion', got: %s", out)
	}

	// Verify it was NOT executed
	for _, executed := range mockDocker.ExecutedCmds {
		if strings.Contains(executed, "rm -rf /") {
			t.Errorf("Dangerous command was executed! %s", executed)
		}
	}

	// 2. Safe Command (Sanity Check)
	respSafe := "I will list files.\n```bash\nls -la\n```"
	outSafe, err := s.ProcessResponse(context.Background(), respSafe)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	if strings.Contains(outSafe, "[BLOCKED]") {
		t.Errorf("Safe command was blocked! %s", outSafe)
	}

	// Verify it WAS executed
	found := false
	for _, executed := range mockDocker.ExecutedCmds {
		if strings.Contains(executed, "ls -la") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Safe command was NOT executed")
	}
}
