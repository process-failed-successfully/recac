package main

import (
	"os"
	"path/filepath"
	"recac/internal/db"
	"strings"
	"testing"
)

func TestRun_Blocker(t *testing.T) {
	// Setup temp DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")

	// 1. Set Blocker
	args := []string{"agent-bridge", "blocker", "Something is wrong"}
	projectID := "test-project"
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Ideally we check DB state, but 'run' just prints to stdout/stderr.
	// We trust SetSignal is covered by db tests. Here we test the CLI wiring.
}

func TestRun_QA(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")

	args := []string{"agent-bridge", "qa"}
	projectID := "test-project"
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func TestRun_Signal(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")

	args := []string{"agent-bridge", "signal", "MY_KEY", "MY_VALUE"}
	projectID := "test-project"
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func TestRun_Manager(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")

	args := []string{"agent-bridge", "manager"}
	projectID := "test-project"
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func TestRun_Verify(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")

	// Create dummy ui_verification.json
	uiPath := "ui_verification.json"
	uiContent := `{
		"requests": [
			{"feature_id": "F1", "instruction": "Check UI", "status": "pending_human"}
		]
	}`
	os.WriteFile(uiPath, []byte(uiContent), 0644)
	defer os.Remove(uiPath)

	args := []string{"agent-bridge", "verify", "F1", "pass"}
	projectID := "test-project"
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Verify file was updated
	data, _ := os.ReadFile(uiPath)
	if !strings.Contains(string(data), `"status": "pass"`) {
		t.Errorf("Expected status to be updated to pass, got: %s", string(data))
	}
}

func TestRun_Feature(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	store, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}) // Fixed SaveFeatures call
	store.SaveFeatures(projectID, `{"project_name": "Test", "features": [{"id": "F1", "name": "Feature 1"}]}`)
	store.Close()

	args := []string{"agent-bridge", "feature", "set", "F1", "--status", "done", "--passes", "true"}
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func TestMainEntry(t *testing.T) {
	// We can't easily test os.Exit(1) without subprocess,
	// but we can at least call main() with valid args to get coverage.
	// We'll use a temp DB and valid args.
	tmpDir := t.TempDir()

	// Backup and restore os.Args and a way to control dbPath in main if possible?
	// main() uses hardcoded ".recac.db". Let's temporarily change CWD.
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"agent-bridge", "qa"}
	main()
}

func TestRun_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	// No args
	if err := run([]string{"agent-bridge"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
		t.Error("Expected error for no args")
	}

	// Unknown command
	if err := run([]string{"agent-bridge", "unknown"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
		t.Error("Expected error for unknown command")
	}

	// verify missing args
	if err := run([]string{"agent-bridge", "verify", "F1"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
		t.Error("Expected error for verify missing args")
	}

	// verify missing file
	if err := run([]string{"agent-bridge", "verify", "F2", "pass"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
		t.Error("Expected error for verify missing file")
	}
}

func TestRun_ClearSignal(t *testing.T) {
	// Need to be in a directory that will contain .recac.db because clear-signal looks in CWD
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	// Pre-populate signal
	store, err := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	store.SetSignal(filepath.Base(tmpDir), "MY_KEY", "MY_VALUE") // Use Base(tmpDir) because clear-signal infers project from dir name
	store.Close()

	args := []string{"agent-bridge", "clear-signal", "MY_KEY"}
	// config passed to run is ignored for clear-signal as it re-opens DB from CWD
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Verify signal cleared
	store, err = db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	defer store.Close()
	val, err := store.GetSignal(filepath.Base(tmpDir), "MY_KEY")
	if err == nil && val != "" {
		t.Errorf("Expected signal to be cleared, got %s", val)
	}
}

func TestRun_Import(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	// Mock Stdin
	input := `{"project_name": "Test", "features": [{"id": "F1", "name": "Feature 1"}]}`
	r, w, _ := os.Pipe()
	w.Write([]byte(input))
	w.Close()

	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	args := []string{"agent-bridge", "import"}
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Verify import
	store, err := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	defer store.Close()
	features, err := store.GetFeatures(projectID)
	if err != nil {
		t.Errorf("Failed to get features: %v", err)
	}
	if !strings.Contains(features, "Feature 1") {
		t.Error("Expected features to be imported")
	}
}

func TestRun_Signal_Privileged(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	keys := []string{"PROJECT_SIGNED_OFF", "TRIGGER_QA", "TRIGGER_MANAGER"}

	for _, key := range keys {
		args := []string{"agent-bridge", "signal", key, "true"}
		if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
			t.Errorf("Expected error for privileged key %s", key)
		} else if !strings.Contains(err.Error(), "privileged") {
			t.Errorf("Expected privileged error message, got: %v", err)
		}
	}
}
