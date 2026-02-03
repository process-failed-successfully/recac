package runner

import (
	"context"
	"log/slog"
	"path/filepath"
	"recac/internal/db"
	"testing"
	"strings"
	"recac/internal/notify"
)

func TestProcessResponse_ReturnsOutputWithBlocker(t *testing.T) {
	mockDocker := &MockDockerClient{
		ExecFunc: func(ctx context.Context, containerID string, cmd []string) (string, error) {
			return "Command Executed", nil
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

	// Set blocker signal
	store.SetSignal("test-project", "BLOCKER", "I am stuck")

	// Response with a command that should run before blocker check
	response := "I will run this command:\n```bash\necho 'running'\n```"

	output, err := s.ProcessResponse(context.Background(), response)
	if err != ErrBlocker {
		t.Fatalf("Expected ErrBlocker, got %v", err)
	}

	if !strings.Contains(output, "Command Executed") {
		t.Errorf("Expected output to contain 'Command Executed', got: %q", output)
	}
}
