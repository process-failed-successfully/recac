package runner

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"recac/internal/notify"
	"testing"
)

func TestProcessResponse_LocalBlocker(t *testing.T) {
	workspace := t.TempDir()

	s := &Session{
		UseLocalAgent: true,
		Workspace:     workspace,
		Logger:        slog.Default(),
		Notifier:      notify.NewManager(func(string, ...interface{}) {}),
		Project:       "test-project-local",
	}

	// Create a blocker file in the workspace
	blockerFile := filepath.Join(workspace, "blockers.txt")
	err := os.WriteFile(blockerFile, []byte("I am blocked by missing dependencies"), 0644)
	if err != nil {
		t.Fatalf("Failed to create blocker file: %v", err)
	}

	// Process response (no commands needed, just checking blocker detection)
	response := "Here is some work.\n```bash\necho 'working...'\n```\nI have identified a blocker."
	output, err := s.ProcessResponse(context.Background(), response)

	if err != ErrBlocker {
		t.Errorf("Expected ErrBlocker, got %v", err)
	}

	if output == "" {
		t.Error("Expected output to be returned even with blocker, got empty string")
	}
}
