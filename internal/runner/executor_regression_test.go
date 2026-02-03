package runner

import (
	"context"
	"log/slog"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/notify"
	"strings"
	"testing"
)

func TestProcessResponse_ReturnsOutputWithBlocker(t *testing.T) {
	mockDocker := &MockDockerClient{
		ExecFunc: func(ctx context.Context, containerID string, cmd []string) (string, error) {
			// Legacy blocker check simulates finding a blocker
			if len(cmd) > 2 && (strings.Contains(cmd[2], "cat recac_blockers.txt") || strings.Contains(cmd[2], "cat blockers.txt")) {
				return "BLOCKER FOUND", nil
			}
			// Regular command execution
			return "Command Executed Successfully", nil
		},
	}

	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	s := &Session{
		Docker:    mockDocker,
		Workspace: workspace,
		DBStore:   store,
		Logger:    slog.Default(),
		Notifier:  notify.NewManager(func(string, ...interface{}) {}),
		Project:   "test-project",
	}

	response := "I will run a command that reveals a blocker.\n```bash\necho 'running command'\n```"

	output, err := s.ProcessResponse(context.Background(), response)

	if err != ErrBlocker {
		t.Errorf("Expected ErrBlocker, got %v", err)
	}

	if !strings.Contains(output, "Command Executed Successfully") {
		t.Errorf("Expected output to contain 'Command Executed Successfully', got: %q", output)
	}
}
