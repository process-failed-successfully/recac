package main

import (
	"path/filepath"
	"recac/internal/db"
	"testing"
)

func TestAgentBridgeRestrictions(t *testing.T) {
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")

	privilegedSignals := []string{
		"PROJECT_SIGNED_OFF",
		"TRIGGER_QA",
		"TRIGGER_MANAGER",
	}
	projectID := "test-project"
	for _, name := range privilegedSignals {
		t.Run("Block_"+name, func(t *testing.T) {
			args := []string{"agent-bridge", "signal", name, "true"}
			err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID)
			if err == nil {
				t.Errorf("Expected error when setting privileged signal %s, got nil", name)
			}
		})
	}

	t.Run("Verify_Missing_File", func(t *testing.T) {
		if err := run([]string{"agent-bridge", "verify", "F2", "pass"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
			t.Error("Expected error for verify missing file")
		}
	})
}
