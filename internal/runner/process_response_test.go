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


func TestSession_ProcessResponse_NoCommands(t *testing.T) {
	s := &Session{
		Docker:   &MockDockerClient{},
		Logger:   slog.Default(),
		Notifier: notify.NewManager(func(string, ...interface{}) {}),
		Project:  "test-project",
	}

	output, err := s.ProcessResponse(context.Background(), "Just some text")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if output != "" {
		t.Errorf("Expected empty output, got %s", output)
	}
}

func TestSession_ProcessResponse_WithCommands(t *testing.T) {
	mockDocker := &MockDockerClient{
		ExecFunc: func(ctx context.Context, containerID string, cmd []string) (string, error) {
			// Check if this is the legacy blocker check
			if len(cmd) > 2 && (strings.Contains(cmd[2], "cat recac_blockers.txt") || strings.Contains(cmd[2], "cat blockers.txt")) {
				return "", nil // No legacy blocker found
			}
			return "Success", nil
		},
	}

	// Create a temporary workspace for blocker file checks
	workspace := t.TempDir()

	// Create DB
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

	response := "Here is code:\n```bash\necho hello\n```"
	output, err := s.ProcessResponse(context.Background(), response)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := "Command Output:\nSuccess\n"
	if output != expected {
		t.Errorf("Expected output containing 'Success', got %s", output)
	}
}

func TestSession_ProcessResponse_Blocker(t *testing.T) {
	mockDocker := &MockDockerClient{
		ExecFunc: func(ctx context.Context, containerID string, cmd []string) (string, error) {
			if len(cmd) > 2 && strings.Contains(cmd[2], "cat recac_blockers.txt") {
				return "", nil
			}
			return "Blocker reported", nil
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

	// Manually set blocker signal to simulate "agent did it"
	store.SetSignal("test-project", "BLOCKER", "I am stuck")

	_, err := s.ProcessResponse(context.Background(), "some commands")
	if err != ErrBlocker {
		t.Errorf("Expected ErrBlocker, got %v", err)
	}
}
