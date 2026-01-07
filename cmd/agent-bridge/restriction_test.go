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
	for _, name := range privilegedSignals {
		t.Run("Block_"+name, func(t *testing.T) {
			args := []string{"agent-bridge", "signal", name, "true"}
			err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
			if err == nil {
				t.Errorf("Expected error when setting privileged signal %s, got nil", name)
			}
		})
	}

	t.Run("Allow_FOO", func(t *testing.T) {
		args := []string{"agent-bridge", "signal", "FOO", "bar"}
		err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
		if err != nil {
			t.Errorf("Unexpected error when setting non-privileged signal FOO: %v", err)
		}
	})
}
