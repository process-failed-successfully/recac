package main

import (
	"os"
	"path/filepath"
	"recac/internal/db"
	"strings"
	"testing"
)

func TestRun_Blocker(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	// 1. Set Blocker
	args := []string{"agent-bridge", "blocker", "Something is wrong"}
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func TestRun_QA(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	args := []string{"agent-bridge", "qa"}
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func TestRun_Signal(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	args := []string{"agent-bridge", "signal", "MY_KEY", "MY_VALUE"}
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func TestRun_Manager(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	args := []string{"agent-bridge", "manager"}
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func TestRun_Verify(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	// Create dummy ui_verification.json
	uiPath := "ui_verification.json"
	uiContent := `{
		"requests": [
			{"feature_id": "F1", "instruction": "Check UI", "status": "pending_human"}
		]
	}`
	// Use tmpDir for ui file? run expects relative path or we change CWD.
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	os.WriteFile(uiPath, []byte(uiContent), 0644)

	args := []string{"agent-bridge", "verify", "F1", "pass"}
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

func TestRun_Import(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	// Mock Stdin
	content := `{"project_name": "ImportTest", "features": [{"id": "Imp1", "name": "Imported Feature"}]}`
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		r.Close()
	}()

	go func() {
		w.Write([]byte(content))
		w.Close()
	}()

	if err := run([]string{"agent-bridge", "import"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Verify imported
	store, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	defer store.Close()
	feats, err := store.GetFeatures(projectID)
	if err != nil {
		t.Fatalf("failed to get features: %v", err)
	}
	if !strings.Contains(feats, "Imported Feature") {
		t.Errorf("Expected imported feature, got: %s", feats)
	}
}

func TestRun_FeatureList(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	store, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	store.SaveFeatures(projectID, `{"project_name": "ListTest", "features": [{"id": "L1", "name": "List Feature"}]}`)
	store.Close()

	// We can't capture stdout easily as run uses fmt.Println directly.
	// But we can verify it doesn't return error.
	if err := run([]string{"agent-bridge", "feature", "list"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func TestRun_FeatureSet_Completion(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	// Setup DB with one feature
	store, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	store.SaveFeatures(projectID, `{"project_name": "Test", "features": [{"id": "F1", "name": "Feature 1", "status": "todo"}]}`)
	store.Close()

	// Run set done
	if err := run([]string{"agent-bridge", "feature", "set", "F1", "--status", "done", "--passes", "true"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Verify COMPLETED signal
	store, _ = db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	defer store.Close()
	sig, err := store.GetSignal(projectID, "COMPLETED")
	if err != nil {
		// Signal might not exist if fail
		t.Logf("GetSignal error: %v", err)
	}
	if sig != "true" {
		t.Errorf("expected COMPLETED signal, got %v", sig)
	}
}

func TestRun_ClearSignal(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project" // clear-signal logic uses directory name as project ID if not passed via env/args correctly?
	// clear-signal command logic:
	// key := args[2]
	// projectPath, err := os.Getwd()
	// dbPath := filepath.Join(projectPath, ".recac.db")
	// projectName := filepath.Base(projectPath)

	// So it relies on CWD.
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Create db file
	store, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	projectName := filepath.Base(tmpDir)
	store.SetSignal(projectName, "MY_SIGNAL", "foo")
	store.Close()

	if err := run([]string{"agent-bridge", "clear-signal", "MY_SIGNAL"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Verify cleared
	store, _ = db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	defer store.Close()
	val, err := store.GetSignal(projectName, "MY_SIGNAL")
	// GetSignal might return empty string instead of error if not found, depending on implementation.
	// Or maybe error "signal not found"
	if err == nil && val != "" {
		t.Errorf("expected error or empty for cleared signal, got val='%s'", val)
	}
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

	// Missing args for various commands
	cmds := [][]string{
		{"agent-bridge", "clear-signal"},
		{"agent-bridge", "blocker"},
		{"agent-bridge", "verify", "id"},
		{"agent-bridge", "signal", "key"},
		{"agent-bridge", "feature", "set"},
		{"agent-bridge", "feature"},
	}
	for _, args := range cmds {
		if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
			t.Errorf("Expected error for missing args: %v", args)
		}
	}

	// Privileged signal
	if err := run([]string{"agent-bridge", "signal", "PROJECT_SIGNED_OFF", "true"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
		t.Error("Expected error for privileged signal")
	}
}

func TestMain_Success(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")

	// Set env
	os.Setenv("RECAC_DB_URL", dbPath)
	defer os.Unsetenv("RECAC_DB_URL")

	// Mock Args
	oldArgs := os.Args
	os.Args = []string{"agent-bridge", "qa"}
	defer func() { os.Args = oldArgs }()

	// Create DB
	store, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	store.Close()

	// Run main
	main()
}
