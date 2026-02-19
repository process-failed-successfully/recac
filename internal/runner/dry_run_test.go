package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSession_DryRun_SkipsExecution(t *testing.T) {
	tmpDir := t.TempDir()
	specContent := "Application Specification"
	specPath := filepath.Join(tmpDir, "app_spec.txt")
	os.WriteFile(specPath, []byte(specContent), 0644)

	// Mock Agent Response with a command
	agentResponse := "Plan: Test Dry Run\n```bash\ntouch executed.txt\n```"
	mockAgent := &MockAgent{Response: agentResponse}

	// Mock Docker Client - using the test-internal MockDockerClient
	execCalled := false
	mockDocker := &MockDockerClient{}
	mockDocker.ExecFunc = func(ctx context.Context, containerID string, cmd []string) (string, error) {
		// Only flag as called if it matches our test command
		// ignore blocker checks etc.
		if len(cmd) > 2 && strings.Contains(cmd[2], "touch executed.txt") {
			execCalled = true
		}
		return "", nil
	}

	// Create Session with DryRun = true
	session := NewSession(mockDocker, mockAgent, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.DryRun = true
	session.MaxIterations = 1 // Ensure loop terminates regardless

	// Run Loop
	ctx := context.Background()
	err := session.RunLoop(ctx)
	if err != nil {
		t.Fatalf("RunLoop failed: %v", err)
	}

	// Assertions
	if execCalled {
		t.Error("DryRun should NOT execute commands on Docker client")
	}

	// Verify executed.txt was NOT created (redundant if mock works, but good sanity check if local exec used)
	if _, err := os.Stat(filepath.Join(tmpDir, "executed.txt")); !os.IsNotExist(err) {
		t.Error("executed.txt should not exist")
	}
}

func TestSession_DryRun_PrintsOutput(t *testing.T) {
	tmpDir := t.TempDir()
	session := NewSession(nil, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.DryRun = true

	cmd := "echo test"
	output, err := session.executeCommandBlock(context.Background(), cmd, 1, 1)

	if err != nil {
		t.Fatalf("executeCommandBlock failed: %v", err)
	}

	expectedMsg := "[DRY RUN] Would execute command block 1/1"
	if !strings.Contains(output, expectedMsg) {
		t.Errorf("Expected output to contain '%s', got '%s'", expectedMsg, output)
	}
}
