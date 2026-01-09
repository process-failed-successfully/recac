package runner

import (
	"os"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"testing"
)

func TestSignalBypass(t *testing.T) {
	t.Setenv("RECAC_TEST_MODE", "false") // Ensure test mode is off by default for this test
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	session := &Session{
		Workspace: workspace,
		Project:   "test-project",
		DBStore:   store,
		Notifier:  notify.NewManager(func(string, ...interface{}) {}),
		Logger:    telemetry.NewLogger(true, ""),
	}

	privilegedSignals := []string{
		"PROJECT_SIGNED_OFF",
		"QA_PASSED",
		"COMPLETED",
		"TRIGGER_QA",
		"TRIGGER_MANAGER",
	}

	for _, name := range privilegedSignals {
		t.Run("Privileged_"+name, func(t *testing.T) {
			// 1. Create signal file
			path := filepath.Join(workspace, name)
			if err := os.WriteFile(path, []byte("true"), 0644); err != nil {
				t.Fatalf("Failed to create signal file: %v", err)
			}
			defer os.Remove(path)

			// 2. Check hasSignal - Should be FALSE for privileged signals
			if session.hasSignal(name) {
				t.Errorf("hasSignal(%s) returned true for filesystem-based signal, expected false", name)
			}

			// 3. Verify it wasn't migrated to DB
			val, _ := store.GetSignal("test-project", name)
			if val != "" {
				t.Errorf("Privileged signal %s was migrated to DB, expected blank", name)
			}
		})
	}

	t.Run("NonPrivileged_FOO", func(t *testing.T) {
		name := "FOO"
		path := filepath.Join(workspace, name)
		if err := os.WriteFile(path, []byte("true"), 0644); err != nil {
			t.Fatalf("Failed to create signal file: %v", err)
		}

		// Check hasSignal - Should be TRUE and MIGRATED
		if !session.hasSignal(name) {
			t.Error("hasSignal(FOO) returned false, expected true (migration)")
		}

		// Verify File Removed
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("Signal file FOO wasn't removed after migration")
		}

		// Verify DB Entry
		val, _ := store.GetSignal("test-project", name)
		if val != "true" {
			t.Errorf("Signal FOO wasn't migrated to DB correctly, got '%s'", val)
		}
	})
}
