package main

import (
	"os"
	"path/filepath"
	"recac/internal/db"
	"strings"
	"testing"
	"time"
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

func TestRun_Ask(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	projectID := "test-project"

	store, _ := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	defer store.Close()

	// Run ask in background
	errCh := make(chan error)
	go func() {
		args := []string{"agent-bridge", "ask", "What is the answer?"}
		errCh <- run(args, db.StoreConfig{Type: "sqlite", ConnectionString: dbPath}, projectID)
	}()

	// Wait a bit for question to be posted
	// Ideally poll DB, but simple sleep works for test
	// We poll DB for QUESTION
	for i := 0; i < 20; i++ {
		q, _ := store.GetSignal(projectID, "QUESTION")
		if q == "What is the answer?" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify question exists
	q, err := store.GetSignal(projectID, "QUESTION")
	if err != nil {
		t.Fatalf("Failed to get question: %v", err)
	}
	if q != "What is the answer?" {
		t.Fatalf("Expected 'What is the answer?', got '%s'", q)
	}

	// Send Answer
	if err := store.SetSignal(projectID, "ANSWER", "42"); err != nil {
		t.Fatalf("Failed to set answer: %v", err)
	}

	// Wait for ask to complete
	// Since loop ticker is 2s, it might take up to 2s.
	// We wait up to 5s.
	// But in test environment, time might move differently? No, real time.
	// We can't easily mock time here without refactoring main.go to accept a ticker.
	// But 2s is acceptable for integration test.

	err = <-errCh
	if err != nil {
		t.Errorf("ask failed: %v", err)
	}

	// Verify signals cleaned up
	q, _ = store.GetSignal(projectID, "QUESTION")
	if q != "" {
		t.Errorf("Question signal not cleaned up")
	}
	a, _ := store.GetSignal(projectID, "ANSWER")
	if a != "" {
		t.Errorf("Answer signal not cleaned up")
	}
}
