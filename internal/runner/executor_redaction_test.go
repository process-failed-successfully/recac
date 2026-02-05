package runner

import (
	"bytes"
	"context"
	"log/slog"
	"recac/internal/notify"
	"recac/internal/security"
	"strings"
	"testing"
)

// MockDockerForRedaction is a simple mock that returns a fixed output containing a secret
type MockDockerForRedaction struct {
	MockDockerClient
	Output string
}

func (m *MockDockerForRedaction) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	cmdStr := strings.Join(cmd, " ")
	// Return empty string for blocker checks to simulate "file not found" or "empty file"
	if strings.Contains(cmdStr, "recac_blockers.txt") || strings.Contains(cmdStr, "blockers.txt") {
		return "", nil
	}

	// Return the secret output for the test command
	return m.Output, nil
}

func TestLogLeak_Reproduction(t *testing.T) {
	secret := "AKIA1234567890123456" // Mock AWS Key
	mockDocker := &MockDockerForRedaction{
		Output: "Current Env: AWS_ACCESS_KEY_ID=" + secret,
	}

	// Capture logs
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	s := &Session{
		Docker:      mockDocker,
		ContainerID: "test-container",
		Notifier:    notify.NewManager(func(string, ...interface{}) {}),
		Logger:      logger,
		Scanner:     security.NewRegexScanner(),
	}

	resp := "Check env.\n```bash\nenv\n```"
	_, err := s.ProcessResponse(context.Background(), resp)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	logOutput := logBuf.String()

	// Verification: The secret should NOT be in the logs
	if strings.Contains(logOutput, secret) {
		t.Fatal("Security Fail: Secret FOUND in logs!")
	}

	// Verification: The secret should be redacted
	if !strings.Contains(logOutput, "[REDACTED]") {
		t.Fatal("Security Fail: [REDACTED] marker NOT found in logs!")
	}

	t.Log("Security Pass: Secret was successfully redacted.")
}
