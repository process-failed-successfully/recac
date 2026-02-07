package runner

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestSession_ProcessResponse_GitCommit_NoChanges(t *testing.T) {
	mockDocker := &MockDockerClient{
		ExecFunc: func(ctx context.Context, containerID string, cmd []string) (string, error) {
			script := strings.Join(cmd, " ")

			// Handle Blocker Check
			if strings.Contains(script, "cat recac_blockers.txt") || strings.Contains(script, "cat blockers.txt") {
				return "", nil // No blocker
			}

			if strings.Contains(script, "git commit") {
				// Simulate git commit failing with exit code 1 due to no changes
				return "On branch main\nnothing to commit, working tree clean", errors.New("exit status 1")
			}
			return "Success", nil
		},
	}

	s := &Session{
		Docker:  mockDocker,
		Logger:  slog.Default(),
		Project: "test-project",
	}

	// Case 1: git commit failure should be ignored and reported as ignored in output
	response := "Attempting to commit:\n```bash\ngit commit -m 'no changes'\n```"
	output, err := s.ProcessResponse(context.Background(), response)

	if err != nil {
		t.Errorf("Expected no error for empty git commit, got: %v", err)
	}

	if !strings.Contains(output, "Ignored Failure") {
		t.Errorf("Expected output to indicate ignored failure, got: %s", output)
	}

	// Case 2: Other failure should be reported in output (ProcessResponse suppresses error return)
	mockDocker.ExecFunc = func(ctx context.Context, containerID string, cmd []string) (string, error) {
		script := strings.Join(cmd, " ")
		if strings.Contains(script, "cat recac_blockers.txt") {
			return "", nil
		}
		return "fatal: some other error", errors.New("exit status 1")
	}

	response = "Attempting to commit:\n```bash\ngit commit -m 'real error'\n```"
	output, err = s.ProcessResponse(context.Background(), response)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !strings.Contains(output, "Command Failed") {
		t.Errorf("Expected output to contain 'Command Failed' for real error, got: %s", output)
	}
}
