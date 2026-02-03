package runner

import (
	"context"
	"log/slog"
	"recac/internal/notify"
	"strings"
	"testing"
)

// We reuse MockDockerForBlocker from blocker_test.go if available.
// If not, we redefine a minimal one here.
// Since Go test compilation merges files in the same package, we should be careful about redeclaration.
// I will verify if I can use it. If redeclaration error occurs, I will rename it.
// To be safe, I'll name it MockDockerForExecutorFix.

type MockDockerForExecutorFix struct {
	DockerClient // Embed interface
	ExecOutput string
	ExecErr    error
}

func (m *MockDockerForExecutorFix) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	// If checking blockers
	if len(cmd) > 2 && strings.Contains(cmd[2], "cat recac_blockers.txt") {
		return "I am blocked!", nil
	}
	// If normal command
	return m.ExecOutput, m.ExecErr
}

func (m *MockDockerForExecutorFix) ExecAsUser(ctx context.Context, id string, user string, cmd []string) (string, error) {
	return m.Exec(ctx, id, cmd)
}

func TestProcessResponse_ReturnsOutputWithBlocker(t *testing.T) {
	ctx := context.Background()
	mockDocker := &MockDockerForExecutorFix{
		ExecOutput: "Command Result 123",
	}

	s := &Session{
		Docker:      mockDocker,
		ContainerID: "test-container",
		Notifier:    notify.NewManager(func(string, ...interface{}) {}),
		Logger:      slog.Default(),
		// UseLocalAgent false means it uses Docker.Exec
		UseLocalAgent: false,
	}

	// Response contains a command block
	response := "Here is a command:\n```bash\necho test\n```"

	// Trigger ProcessResponse
	// We expect executeCommandBlock to run (returning "Command Result 123")
	// Then checkBlockers runs (mocked to find "I am blocked!")
	// It should return "Command Output:\nCommand Result 123\n" AND ErrBlocker

	output, err := s.ProcessResponse(ctx, response)

	if err != ErrBlocker {
		t.Errorf("Expected ErrBlocker, got %v", err)
	}

	if !strings.Contains(output, "Command Result 123") {
		t.Errorf("Expected output to contain 'Command Result 123', got %q", output)
	}
}
