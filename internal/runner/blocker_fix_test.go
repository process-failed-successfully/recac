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

func TestSession_ProcessResponse_BlockerPreservesOutput(t *testing.T) {
	mockDocker := &MockDockerClient{
		ExecFunc: func(ctx context.Context, containerID string, cmd []string) (string, error) {
			// Simulate successful command execution
			if len(cmd) > 2 && strings.Contains(cmd[2], "echo 'I did something'") {
				return "I did something", nil
			}
			return "", nil
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

	// Manually set blocker signal
	store.SetSignal("test-project", "BLOCKER", "I am stuck")

	response := "I will do this:\n```bash\necho 'I did something'\n```"
	output, err := s.ProcessResponse(context.Background(), response)

	if err != ErrBlocker {
		t.Errorf("Expected ErrBlocker, got %v", err)
	}

	if !strings.Contains(output, "I did something") {
		t.Errorf("Expected output to contain 'I did something', got: %q", output)
	}
}
