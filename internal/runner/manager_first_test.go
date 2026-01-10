package runner

import (
	"os"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"testing"
)

func TestSelectPrompt_ManagerFirst(t *testing.T) {
	// Setup
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	session := &Session{
		Workspace:        workspace,
		DBStore:          store,
		ManagerFirst:     true,
		Iteration:        1,
		ManagerFrequency: 5,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
	}

	// Test Iteration 1 with ManagerFirst=true
	_, _, isManager, err := session.SelectPrompt()
	if err != nil {
		t.Fatalf("SelectPrompt failed: %v", err)
	}

	if !isManager {
		t.Error("Expected isManager=true for ManagerFirst iteration 1")
	}

	// We expect the prompt to contain "Manager" or "Review" (depending on template)
	// Currently relying on logical isManager check is sufficient for flow verification
}

func TestSelectPrompt_NormalFirst(t *testing.T) {
	// Setup
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	// Create dummy spec file
	if err := os.WriteFile(filepath.Join(workspace, "app_spec.txt"), []byte("test spec"), 0644); err != nil {
		t.Fatal(err)
	}

	session := &Session{
		Workspace:        workspace,
		DBStore:          store,
		ManagerFirst:     false,
		Iteration:        1,
		SpecFile:         "app_spec.txt",
		ManagerFrequency: 5,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
	}

	// Test Iteration 1 with ManagerFirst=false
	_, _, isManager, err := session.SelectPrompt()
	if err != nil {
		t.Fatalf("SelectPrompt failed: %v", err)
	}

	if isManager {
		t.Error("Expected isManager=false for Normal iteration 1")
	}
}
