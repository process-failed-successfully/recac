package main

import (
	"path/filepath"
	"recac/internal/db"
	"testing"
)

func TestAgentBridgeRestrictions(t *testing.T) {
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")

	// Privileged signals that MUST be blocked
	privilegedSignals := []string{
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

	// Allowed signals (PROJECT_SIGNED_OFF should now be allowed)
	allowedSignals := []string{
		"PROJECT_SIGNED_OFF",
	}
	for _, name := range allowedSignals {
		t.Run("Allow_"+name, func(t *testing.T) {
			args := []string{"agent-bridge", "signal", name, "true"}
			err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID)
			if err != nil {
				t.Errorf("Expected success when setting signal %s, got error: %v", name, err)
			}
		})
	}

	t.Run("Verify_Missing_File", func(t *testing.T) {
		if err := run([]string{"agent-bridge", "verify", "F2", "pass"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
			t.Error("Expected error for verify missing file")
		}
	})
}
