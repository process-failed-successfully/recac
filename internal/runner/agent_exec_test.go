package runner

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

type MockDockerForExec struct {
	DockerClient
	ExecutedCmds []string
	ExecDelay    time.Duration
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
	}

	// 1. Test standard block
	resp1 := "I will create a file.\n```bash\necho 'hello' > test.txt\n```"
	out1, err := s.ProcessResponse(context.Background(), resp1)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	if !strings.Contains(out1, "Success: /bin/sh -c echo 'hello' > test.txt") {
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
	resp3 := "I need sudo:\n```bash\nsudo apt-get update\n```"
	out3, err := s.ProcessResponse(context.Background(), resp3)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	if !strings.Contains(out3, "sudo apt-get update") {
		t.Errorf("Sudo command not found: %s", out3)
	}

	// Verify executed commands match what we expect
	expectedCmds := []string{
		"/bin/sh -c echo 'hello' > test.txt",
		"/bin/sh -c echo 1",
		"/bin/sh -c echo 2",
		"/bin/sh -c sudo apt-get update",
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
