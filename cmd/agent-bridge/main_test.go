package main

import (
	"bytes"
	"io"
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

func TestRun_Manager(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")

	args := []string{"agent-bridge", "manager"}
	projectID := "test-project"
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func TestRun_Signal(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	// Valid signal
	args := []string{"agent-bridge", "signal", "MY_KEY", "MY_VALUE"}
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Privileged signal (should fail)
	privilegedArgs := []string{"agent-bridge", "signal", "TRIGGER_QA", "true"}
	if err := run(privilegedArgs, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
		t.Error("Expected error for privileged signal, got nil")
	} else if !strings.Contains(err.Error(), "privileged") {
		t.Errorf("Expected privileged error message, got: %v", err)
	}
}

func TestRun_ClearSignal(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")

	// Pre-create DB and set a signal using store directly
	store, _ := db.NewSQLiteStore(dbPath)
	store.SetSignal("test-project", "MY_SIGNAL", "some value") // Project ID in CLI is used, but clear-signal derives it from path?
	// clear-signal logic: projectName = filepath.Base(projectPath)
	// If we are in tmpDir, project name is Base(tmpDir).
	projectName := filepath.Base(tmpDir)
	store.SetSignal(projectName, "MY_SIGNAL", "some value")
	store.Close()

	// Need to change CWD to tmpDir because clear-signal looks for .recac.db in CWD
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	args := []string{"agent-bridge", "clear-signal", "MY_SIGNAL"}
	// Config passed to run is ignored by clear-signal which re-opens SQLite
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: ".recac.db"}, "ignored"); err != nil {
		t.Fatalf("run clear-signal failed: %v", err)
	}

	// Verify signal is gone
	store, _ = db.NewSQLiteStore(dbPath)
	val, err := store.GetSignal(projectName, "MY_SIGNAL")
	store.Close()
	if val != "" || err == nil {
		// GetSignal typically returns empty string and/or error if not found?
		// Check db implementation. Usually GetSignal returns empty string if not found, or error.
		// If implementation returns error on not found, err should be non-nil.
		// If it returns empty string, val should be empty.
		// Assuming GetSignal returns "" if not found.
		if val != "" {
			t.Errorf("Expected signal MY_SIGNAL to be cleared, got: %s", val)
		}
	}
}

func TestRun_Verify(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")

	// Create dummy ui_verification.json
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	uiPath := "ui_verification.json"
	uiContent := `{
		"requests": [
			{"feature_id": "F1", "instruction": "Check UI", "status": "pending_human"}
		]
	}`
	os.WriteFile(uiPath, []byte(uiContent), 0644)

	args := []string{"agent-bridge", "verify", "F1", "pass"}
	projectID := "test-project"
	// verify reads ui_verification.json from CWD
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
	store.SaveFeatures(projectID, `{"project_name": "Test", "features": [{"id": "F1", "name": "Feature 1", "status": "todo", "passes": false}]}`)
	store.Close()

	// Feature List
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	argsList := []string{"agent-bridge", "feature", "list"}
	if err := run(argsList, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		w.Close()
		os.Stdout = oldStdout
		t.Fatalf("run feature list failed: %v", err)
	}
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	if !strings.Contains(buf.String(), "F1") {
		t.Errorf("Expected feature list output to contain F1, got: %s", buf.String())
	}

	// Feature Set
	argsSet := []string{"agent-bridge", "feature", "set", "F1", "--status", "done", "--passes", "true"}
	if err := run(argsSet, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run feature set failed: %v", err)
	}

	// Verify completion signal
	// Re-open store to check
	store, _ = db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	val, _ := store.GetSignal(projectID, "COMPLETED")
	store.Close()
	if val != "true" {
		t.Error("Expected COMPLETED signal to be true after finishing all features")
	}
}

func TestRun_Import(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	// Mock Stdin
	content := `{"project_name": "Imported", "features": [{"id": "I1", "name": "Imported 1"}]}`
	tmpFile := filepath.Join(tmpDir, "features.json")
	os.WriteFile(tmpFile, []byte(content), 0644)

	oldStdin := os.Stdin
	f, _ := os.Open(tmpFile)
	os.Stdin = f
	defer func() {
		f.Close()
		os.Stdin = oldStdin
	}()

	args := []string{"agent-bridge", "import"}
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run import failed: %v", err)
	}

	// Verify in DB
	store, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	feats, _ := store.GetFeatures(projectID)
	store.Close()
	if !strings.Contains(feats, "Imported 1") {
		t.Errorf("Expected imported feature to be in DB")
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
	// Env vars
	os.Setenv("RECAC_DB_URL", ".recac.db")
	os.Setenv("RECAC_PROJECT_ID", "main-test")
	defer os.Unsetenv("RECAC_DB_URL")
	defer os.Unsetenv("RECAC_PROJECT_ID")

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

	// feature unknown
	if err := run([]string{"agent-bridge", "feature", "unknown"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
		t.Error("Expected error for feature unknown subcommand")
	}

	// verify missing args
	if err := run([]string{"agent-bridge", "verify", "F1"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
		t.Error("Expected error for verify missing args")
	}

	// verify missing file
	// Note: verify test above creates the file. Here we assume it doesn't exist in tmpDir/sub if we use new dir
	tmpDir2 := t.TempDir()
	// Change WD to somewhere with no json
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir2)
	defer os.Chdir(oldWd)

	if err := run([]string{"agent-bridge", "verify", "F2", "pass"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
		t.Error("Expected error for verify missing file")
	}
}
