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
	// Ensure we don't mess up current dir
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	uiContent := `{
		"requests": [
			{"feature_id": "F1", "instruction": "Check UI", "status": "pending_human"}
		]
	}`
	os.WriteFile(uiPath, []byte(uiContent), 0644)

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

	store, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	store.SaveFeatures(projectID, `{"project_name": "Test", "features": [{"id": "F1", "name": "Feature 1"}]}`)
	store.Close()

	args := []string{"agent-bridge", "feature", "set", "F1", "--status", "done", "--passes", "true"}
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func TestMainEntry(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"agent-bridge", "qa"}
	// This calls main() which calls os.Exit(1) on failure.
	// Since we set up env correctly (defaults), it should pass and exit normally?
	// main() calls run(). If run() returns nil, main() finishes.
	// But main() doesn't call os.Exit(0).
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
}

func TestRun_ClearSignal(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	// Setup DB in tmpDir (because clear-signal looks in CWD)
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	store.SetSignal(filepath.Base(tmpDir), "KEY", "VALUE")
	store.Close()

	// args: agent-bridge clear-signal KEY
	args := []string{"agent-bridge", "clear-signal", "KEY"}

	config := db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}

	if err := run(args, config, "ignored"); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Verify signal cleared
	store, _ = db.NewSQLiteStore(dbPath)
	defer store.Close()
	val, err := store.GetSignal(filepath.Base(tmpDir), "KEY")
	if err != nil {
		t.Errorf("GetSignal failed: %v", err)
	}
	if val != "" {
		t.Errorf("Signal not cleared, got: %q", val)
	}
}

func TestRun_Import(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")

	jsonContent := `{"features": [{"id": "F_IMP", "name": "Imported"}]}`
	origInput := inputReader
	inputReader = strings.NewReader(jsonContent)
	defer func() { inputReader = origInput }()

	args := []string{"agent-bridge", "import"}
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, "test-proj"); err != nil {
		t.Fatalf("run import failed: %v", err)
	}

	// Verify import
	store, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	defer store.Close()
	content, _ := store.GetFeatures("test-proj")
	if !strings.Contains(content, "F_IMP") {
		t.Errorf("Feature not imported")
	}
}

func TestRun_FeatureList(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	store.SaveFeatures("test-proj", `{"features": [{"id": "F_LIST", "name": "Listed"}]}`)
	store.Close()

	args := []string{"agent-bridge", "feature", "list"}
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, "test-proj"); err != nil {
		t.Fatalf("run feature list failed: %v", err)
	}
}
