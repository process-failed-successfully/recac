package runner

import (
	"context"
	"fmt"
	"log/slog"
	"recac/internal/notify"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

type MockDockerForExec struct {
	DockerClient
	ExecutedCmds   []string
	ExecDelay      time.Duration
	SignalCallback func(key, value string)
}

func (m *MockDockerForExec) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	fullCmd := strings.Join(cmd, " ")
	m.ExecutedCmds = append(m.ExecutedCmds, fullCmd)

	if m.ExecDelay > 0 {
		select {
		case <-time.After(m.ExecDelay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	// Simulate success for most commands
	if strings.Contains(fullCmd, "fail") {
		return "", fmt.Errorf("simulated failure")
	}
	// Blocker checks should be empty unless we want to test blockers
	if strings.Contains(fullCmd, "cat recac_blockers.txt") || strings.Contains(fullCmd, "cat blockers.txt") {
		return "", nil
	}

	// Handle agent-bridge signals for UI tests
	// Simulate success for signal commands so the session doesn't block on them?
	// But in tests, we need the session to actually see the signal if it checks for it via DB.
	// Since MockDockerForExec doesn't have access to the DBStore of the session (it's decoupled),
	// we assume the test might inject a DB-aware mock or we just return success.
	// However, TestSession_RunLoop_UIVerification relies on the signal effectively terminating the loop?
	// The loop terminates if MaxIterations reached OR if project complete.
	// We return success here. The Session RunLoop calls `session.hasSignal` which checks DBStore.
	// If MockDocker doesn't update DBStore, then `hasSignal` remains false.
	// That explains why it loops forever!
	// We need MockDockerForExec to be aware of the store if we want end-to-end signal testing without real binary.

	// BUT, `agent-bridge signal ...` is executed inside the container.
	// In a real run, the binary connects to DB and updates it.
	// In this mock, we just say "Success".
	// The session.RunLoop doesn't re-check DB instantly after exec unless we tell it to?
	// Actually, `checkCompletion` checks signals.
	// If `agent-bridge signal QA_PASSED true` is executed, the mock returns "Success".
	// The DB is NOT updated.
	// So `session.hasSignal("QA_PASSED")` returns false.
	// So the loop continues.

	// FIX: We need to intercept "agent-bridge signal" commands in this mock and update a map or store if injected.
	// MockDockerForExec needs a way to callback or update state.
	if strings.Contains(fullCmd, "agent-bridge signal") && m.SignalCallback != nil {
		parts := strings.Fields(fullCmd)
		// expected: ... agent-bridge signal KEY VALUE
		// Find 'signal' and take next two
		for i, p := range parts {
			if p == "signal" && i+2 < len(parts) {
				m.SignalCallback(parts[i+1], parts[i+2])
				break
			}
		}
	}

	return "Success: " + fullCmd, nil
}

func (m *MockDockerForExec) ExecAsUser(ctx context.Context, id string, user string, cmd []string) (string, error) {
	return m.Exec(ctx, id, cmd)
}

func TestSession_ProcessResponse_Thorough(t *testing.T) {
	mockDocker := &MockDockerForExec{}
	s := &Session{
		Docker:      mockDocker,
		ContainerID: "test-container",
		Notifier:    notify.NewManager(func(string, ...interface{}) {}),
		Logger:      slog.Default(),
	}

	// 1. Test standard block
	resp1 := "I will create a file.\n```bash\necho 'hello' > test.txt\n```"
	out1, err := s.ProcessResponse(context.Background(), resp1)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	if !strings.Contains(out1, "Success: /bin/bash -c echo 'hello' > test.txt") {
		t.Errorf("Output missing expected command: %s", out1)
	}

	// 2. Test multiple blocks with varying whitespace (testing our new regex)
	resp2 := "Multiple blocks:\n```bash \necho 1\n```\nAnd:\n```bash\necho 2```"
	out2, err := s.ProcessResponse(context.Background(), resp2)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	if strings.Count(out2, "Success:") != 2 {
		t.Errorf("Expected 2 successful commands, got: %s", out2)
	}

	// 3. Test sudo blocks
	resp3 := "I need to install something:\n```bash\napk add curl\n```"
	out3, err := s.ProcessResponse(context.Background(), resp3)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	if !strings.Contains(out3, "apk add curl") {
		t.Errorf("Apk command not found: %s", out3)
	}

	// Verify executed commands match what we expect
	expectedCmds := []string{
		"/bin/bash -c echo 'hello' > test.txt",
		"/bin/bash -c echo 1",
		"/bin/bash -c echo 2",
		"/bin/bash -c apk add curl",
	}

	for i, cmd := range expectedCmds {
		// Note: legacy check for blockers adds commands, so we look for our specific ones
		found := false
		for _, executed := range mockDocker.ExecutedCmds {
			if strings.Contains(executed, cmd) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected command %d not found in execution log: %s", i, cmd)
		}
	}
}

func TestSession_ProcessResponse_Timeout(t *testing.T) {
	// Set short timeout for test
	viper.Set("bash_timeout", 1) // 1 second
	defer viper.Set("bash_timeout", 120)

	mockDocker := &MockDockerForExec{
		ExecDelay: 2 * time.Second, // Delay longer than timeout
	}
	s := &Session{
		Docker:      mockDocker,
		ContainerID: "test-container",
		Notifier:    notify.NewManager(func(string, ...interface{}) {}),
		Logger:      slog.Default(),
	}

	resp := "Wait for it...\n```bash\necho 'slow command'\n```"
	out, _ := s.ProcessResponse(context.Background(), resp)

	// The Exec function should receive a cancelled context and return ctx.Err()
	// ProcessResponse catches this and prints "Command Failed... Command timed out"

	if !strings.Contains(out, "Command timed out after 1 seconds") {
		t.Errorf("Expected output to contain timeout message, got: %s", out)
	}
	if !strings.Contains(out, "Command Failed") {
		t.Errorf("Expected command to be marked as failed")
	}
}
