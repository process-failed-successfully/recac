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

func TestSession_ProcessResponse_ExecutesCommandsBeforeBlockerCheck(t *testing.T) {
	// This test verifies if commands are executed even when a blocker signal is present.
	// In the regression, commands ARE executed because checkBlockers is called after execution.
	// If this behavior is undesired, this test helps confirm the current state.

	executed := false
	mockDocker := &MockDockerClient{
		ExecFunc: func(ctx context.Context, containerID string, cmd []string) (string, error) {
			// If we see the specific command, mark as executed
			if len(cmd) > 2 && strings.Contains(cmd[2], "echo 'I executed'") {
				executed = true
				return "I executed", nil
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

	// 1. Set a blocker signal
	store.SetSignal("test-project", "BLOCKER", "I am stuck")

	// 2. Provide a response with a command
	response := "I will try to run this:\n```bash\necho 'I executed'\n```"

	// 3. Process Response
	_, err := s.ProcessResponse(context.Background(), response)

	// 4. Verify results
	if err != ErrBlocker {
		t.Errorf("Expected ErrBlocker, got %v", err)
	}

	// 5. Check if command was executed
	if executed {
		t.Errorf("Command WAS executed despite blocker (FIX FAILED)")
	} else {
		t.Log("Command WAS NOT executed (Desired Behavior - FIX VERIFIED)")
	}
}
