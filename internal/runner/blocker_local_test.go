package runner

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"recac/internal/notify"
	"testing"
)

func TestCheckBlockers_Local(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	s := &Session{
		UseLocalAgent: true,
		Workspace:     tmpDir,
		Notifier:      notify.NewManager(func(string, ...interface{}) {}),
		Logger:        slog.Default(),
	}

	// 1. No blockers
	if err := s.checkBlockers(ctx); err != nil {
		t.Errorf("Expected no blockers, got error: %v", err)
	}

	// 2. Real Blocker in recac_blockers.txt
	blockerFile := filepath.Join(tmpDir, "recac_blockers.txt")
	os.WriteFile(blockerFile, []byte("I am blocked by missing API key"), 0644)

	err := s.checkBlockers(ctx)
	if err != ErrBlocker {
		t.Errorf("Expected ErrBlocker for real blocker, got: %v", err)
	}

	// Cleanup
	os.Remove(blockerFile)

	// 3. False positive
	os.WriteFile(blockerFile, []byte("No blockers"), 0644)
	if err := s.checkBlockers(ctx); err != nil {
		t.Errorf("Expected no error for false positive, got: %v", err)
	}
	// Verify file was deleted
	if _, err := os.Stat(blockerFile); !os.IsNotExist(err) {
		t.Error("Expected false positive blocker file to be deleted")
	}
}
