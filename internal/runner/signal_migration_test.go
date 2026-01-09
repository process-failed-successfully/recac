package runner

import (
	"os"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"testing"
)

func TestSignalMigration(t *testing.T) {
	// 1. Setup Workspace
	workspace := t.TempDir()

	// 2. Setup DB
	dbPath := filepath.Join(workspace, ".recac.db")
	store, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 3. Create Session
	// Note: Agent and Docker are nil as we only test hasSignal logic
	session := &Session{
		Workspace: workspace,
		Project:   "test-project",
		DBStore:   store,
		Notifier:  notify.NewManager(func(string, ...interface{}) {}),
		Logger:    telemetry.NewLogger(true, ""),
	}

	// 4. Test Scenario: Agent creates a signal file
	signalName := "TEST_SIGNAL"
	signalPath := filepath.Join(workspace, signalName)
	if err := os.WriteFile(signalPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create mock signal file: %v", err)
	}

	// 5. Check hasSignal - Should return true AND migrate
	if !session.hasSignal(signalName) {
		t.Error("hasSignal returned false, expected true (migration)")
	}

	// 6. Verify File Removed
	if _, err := os.Stat(signalPath); !os.IsNotExist(err) {
		t.Error("Signal file wasn't removed after migration")
	}

	// 7. Verify DB Entry
	val, err := store.GetSignal("test-project", signalName)
	if err != nil {
		t.Errorf("DB check failed: %v", err)
	}
	if val != "true" {
		t.Errorf("DB signal value expected 'true', got '%s'", val)
	}

	// 8. Check hasSignal again - Should still be true (from DB)
	if !session.hasSignal(signalName) {
		t.Error("hasSignal returned false on second check (DB), expected true")
	}
}
