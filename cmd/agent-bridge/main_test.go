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
	// Create in current dir as main.go expects it there?
	// main.go reads "ui_verification.json" from CWD.
	// So we need to change CWD or use absolute path if main.go supports it.
	// main.go uses hardcoded "ui_verification.json".
	// So we must change CWD.
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

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

	// Init store with some features
	store, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	store.SaveFeatures(projectID, `{"project_name": "Test", "features": [{"id": "F1", "name": "Feature 1", "status": "todo", "passes": false}]}`)
	store.Close()

	// 1. Test Set
	args := []string{"agent-bridge", "feature", "set", "F1", "--status", "done", "--passes", "true"}
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run set failed: %v", err)
	}

	// 2. Test List
	// We need to capture stdout to verify output.
	// run writes to fmt.Println.
	// We can use a pipe.
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	os.Stdout = w

	argsList := []string{"agent-bridge", "feature", "list"}
	if err := run(argsList, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run list failed: %v", err)
	}

	w.Close()
	out, _ := io.ReadAll(r)
	if !strings.Contains(string(out), "F1") {
		t.Errorf("Expected output to contain F1, got: %s", string(out))
	}
}

func TestRun_Import(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	// Mock Stdin
	content := `{"project_name": "Imported", "features": [{"id": "I1", "name": "Imported Feature"}]}`
	tmpFile, err := os.CreateTemp(tmpDir, "stdin")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.WriteString(content)
	tmpFile.Seek(0, 0)

	oldStdin := os.Stdin
	defer func() {
		os.Stdin = oldStdin
		tmpFile.Close()
	}()
	os.Stdin = tmpFile

	args := []string{"agent-bridge", "import"}
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err != nil {
		t.Fatalf("run import failed: %v", err)
	}

	// Verify import
	store, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	features, _ := store.GetFeatures(projectID)
	store.Close()

	if !strings.Contains(features, "I1") {
		t.Errorf("Expected imported feature I1 in DB, got: %s", features)
	}
}

func TestRun_ClearSignal(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")

	// Need to be in project root for clear-signal as it looks for .recac.db in CWD
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Set a signal first
	store, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	store.SetSignal("test-project", "MY_SIGNAL", "some value")
	// Note: clear-signal uses project name from directory base name as project ID?
	// The code says:
	// projectName := filepath.Base(projectPath)
	// sqliteStore.DeleteSignal(projectName, key)
	// So we need to match projectID used in SetSignal with directory name.
	// But tmpDir name is random.
	// Let's create a specific directory "myproject" inside tmpDir.
	projectDir := filepath.Join(tmpDir, "myproject")
	os.Mkdir(projectDir, 0755)
	os.Chdir(projectDir)

	// The DB must be in project root (.recac.db)
	dbPathProject := filepath.Join(projectDir, ".recac.db")

	store2, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPathProject})
	store2.SetSignal("myproject", "MY_SIGNAL", "value")
	store2.Close()

	args := []string{"agent-bridge", "clear-signal", "MY_SIGNAL"}
	// Config passed to run might be ignored by clear-signal which re-opens DB from CWD.
	if err := run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPathProject}, "ignored"); err != nil {
		t.Fatalf("run clear-signal failed: %v", err)
	}

	// Verify
	store3, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPathProject})
	val, err := store3.GetSignal("myproject", "MY_SIGNAL")
	store3.Close()

	if err == nil && val != "" {
		t.Errorf("Signal should be cleared, got: %s", val)
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

	// Create DB first
	dbPath := ".recac.db"
	store, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	store.Close()

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"agent-bridge", "qa"}
	// This calls run() which we tested separately, but covers main() logic.
	// We need to avoid os.Exit(1) if run fails.
	// run() should succeed here.
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
	// We need to change CWD to where ui_verification.json is NOT present
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	if err := run([]string{"agent-bridge", "verify", "F2", "pass"}, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID); err == nil {
		t.Error("Expected error for verify missing file")
	}
}
