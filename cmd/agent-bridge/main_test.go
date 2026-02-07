package main

import (
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

	store, err := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := store.SaveFeatures(projectID, `{"project_name": "Test", "features": [{"id": "F1", "name": "Feature 1"}]}`); err != nil {
		store.Close()
		t.Fatalf("failed to save features: %v", err)
	}
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
	main()
}

func TestRun_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	if err := run([]string{"agent-bridge"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
		t.Error("Expected error for no args")
	}

	if err := run([]string{"agent-bridge", "unknown"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
		t.Error("Expected error for unknown command")
	}

	if err := run([]string{"agent-bridge", "verify", "F1"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
		t.Error("Expected error for verify missing args")
	}

	if err := run([]string{"agent-bridge", "verify", "F2", "pass"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
		t.Error("Expected error for verify missing file")
	}
}

func TestRun_FeatureList(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	store, err := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := store.SaveFeatures(projectID, `{"project_name": "Test", "features": [{"id": "F1", "name": "Feature 1"}]}`); err != nil {
		store.Close()
		t.Fatalf("failed to save features: %v", err)
	}
	store.Close()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	args := []string{"agent-bridge", "feature", "list"}
	err = run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	out, _ := io.ReadAll(r)
	if !strings.Contains(string(out), "Feature 1") {
		t.Errorf("Expected output to contain 'Feature 1', got: %s", string(out))
	}
}

func TestRun_ClearSignal(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	// The CLI uses the directory base name as project ID
	projectID := filepath.Base(tmpDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	store, err := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := store.SetSignal(projectID, "TEST_SIGNAL", "value"); err != nil {
		store.Close()
		t.Fatalf("failed to set signal: %v", err)
	}
	store.Close()

	args := []string{"agent-bridge", "clear-signal", "TEST_SIGNAL"}
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Verify signal is cleared
	store, err = db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	if err != nil {
		t.Fatalf("failed to reopen store: %v", err)
	}
	defer store.Close()

	val, err := store.GetSignal(projectID, "TEST_SIGNAL")
	// If it was cleared, we expect either error or empty string depending on implementation.
	// Assuming GetSignal returns error if not found.
	if err == nil && val != "" {
		t.Errorf("Expected signal to be cleared, but got: %s", val)
	}
}

func TestRun_Import(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	content := `{"project_name": "Imported", "features": [{"id": "I1", "name": "Imported Feature"}]}`
	tmpFile := filepath.Join(tmpDir, "features.json")
	os.WriteFile(tmpFile, []byte(content), 0644)

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	f, _ := os.Open(tmpFile)
	defer f.Close()
	os.Stdin = f

	args := []string{"agent-bridge", "import"}
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	store, err := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	if err != nil {
		t.Fatalf("failed to reopen store: %v", err)
	}
	defer store.Close()
	features, _ := store.GetFeatures(projectID)
	if !strings.Contains(features, "Imported Feature") {
		t.Errorf("Expected imported feature in DB")
	}
}
