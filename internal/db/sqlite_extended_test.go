package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSQLiteStore_Error(t *testing.T) {
	// 1. Test with a directory that doesn't exist and can't be created (e.g. invalid chars or permissions)
	// Actually, sqlite driver might create the file.
	// Let's try to pass a directory as the file path.
	tmpDir := t.TempDir()

	// Create a store where the path is an existing directory
	_, err := NewSQLiteStore(tmpDir)
	if err == nil {
		t.Error("Expected error when opening a directory as sqlite db, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}

	// 2. Test with a read-only directory
	// Create a subdir
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.Mkdir(readOnlyDir, 0555); err != nil { // Read/Execute only
		t.Fatal(err)
	}

	// Try to create db inside read-only dir
	dbPath := filepath.Join(readOnlyDir, "test.db")
	_, err = NewSQLiteStore(dbPath)
	if err == nil {
		// Note: Root often ignores permissions, so this might pass in some envs (like Docker as root).
		// If it passes, we can't easily test this failure mode without mocks.
		// We'll just log it.
		t.Log("Warning: Could not induce failure with read-only directory (running as root?)")
	} else {
		t.Logf("Got expected error or verified permissions: %v", err)
	}
}

func TestSQLiteStore_Cleanup_Extended(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "cleanup_test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	projectID := "proj1"
	agentID := "agent1"

	// 1. Test Expired Locks Cleanup
	// Insert an expired lock manually using raw SQL to bypass AcquireLock validation
	// SQLite current_timestamp is UTC.
	_, err = store.db.Exec(`INSERT INTO file_locks (project_id, path, agent_id, expires_at) VALUES (?, ?, ?, datetime('now', '-1 hour'))`,
		projectID, "/expired/path", agentID)
	if err != nil {
		t.Fatalf("Failed to insert expired lock: %v", err)
	}

	// Insert a valid lock
	_, err = store.db.Exec(`INSERT INTO file_locks (project_id, path, agent_id, expires_at) VALUES (?, ?, ?, datetime('now', '+1 hour'))`,
		projectID, "/valid/path", agentID)
	if err != nil {
		t.Fatalf("Failed to insert valid lock: %v", err)
	}

	// 2. Test Old Signals Cleanup
	// Insert old signal
	_, err = store.db.Exec(`INSERT INTO signals (project_id, key, value, created_at) VALUES (?, ?, ?, datetime('now', '-25 hours'))`,
		projectID, "old_key", "val")
	if err != nil {
		t.Fatalf("Failed to insert old signal: %v", err)
	}

	// Insert recent signal
	_, err = store.db.Exec(`INSERT INTO signals (project_id, key, value, created_at) VALUES (?, ?, ?, datetime('now', '-1 hour'))`,
		projectID, "recent_key", "val")
	if err != nil {
		t.Fatalf("Failed to insert recent signal: %v", err)
	}

	// Insert critical old signal
	_, err = store.db.Exec(`INSERT INTO signals (project_id, key, value, created_at) VALUES (?, ?, ?, datetime('now', '-25 hours'))`,
		projectID, "COMPLETED", "true")
	if err != nil {
		t.Fatalf("Failed to insert critical signal: %v", err)
	}

	// 3. Run Cleanup
	if err := store.Cleanup(); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// 4. Verify
	// Check locks
	locks, err := store.GetActiveLocks(projectID)
	if err != nil {
		t.Fatalf("GetActiveLocks failed: %v", err)
	}
	if len(locks) != 1 {
		t.Errorf("Expected 1 active lock, got %d", len(locks))
	} else if locks[0].Path != "/valid/path" {
		t.Errorf("Expected valid lock to remain, got %s", locks[0].Path)
	}

	// Check signals
	val, _ := store.GetSignal(projectID, "old_key")
	if val != "" {
		t.Errorf("Expected old_key to be deleted")
	}
	val, _ = store.GetSignal(projectID, "recent_key")
	if val == "" {
		t.Errorf("Expected recent_key to remain")
	}
	val, _ = store.GetSignal(projectID, "COMPLETED")
	if val == "" {
		t.Errorf("Expected critical COMPLETED signal to remain")
	}
}

func TestSQLiteStore_UpdateFeatureStatus_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "features_test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Update when project doesn't exist/has no features
	err = store.UpdateFeatureStatus("nonexistent_proj", "f1", "done", true)
	if err == nil {
		t.Error("Expected error for nonexistent project features, got nil")
	}

	// Save features but try to update nonexistent feature ID
	featuresJSON := `{"project_name":"Test","features":[{"id":"F1","status":"pending"}]}`
	store.SaveFeatures("proj1", featuresJSON)

	err = store.UpdateFeatureStatus("proj1", "F99", "done", true)
	if err == nil {
		t.Error("Expected error for nonexistent feature ID, got nil")
	} else if err.Error() != "feature ID F99 not found" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestSQLiteStore_GetActiveLocks_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "empty_locks.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	locks, err := store.GetActiveLocks("proj1")
	if err != nil {
		t.Fatalf("GetActiveLocks failed on empty DB: %v", err)
	}
	if len(locks) != 0 {
		t.Errorf("Expected 0 locks, got %d", len(locks))
	}
}

// TestLocking_Expiration_Race simulates expiration check logic
func TestSQLiteStore_AcquireLock_Expired_Highjack(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "lock_race.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	projectID := "proj1"
	path := "/file.txt"
	agent1 := "agent1"
	agent2 := "agent2"

	// 1. Agent 1 acquires lock
	success, err := store.AcquireLock(projectID, path, agent1, 1*time.Second)
	if !success || err != nil {
		t.Fatalf("Agent1 failed to acquire lock: %v", err)
	}

	// 2. Force expire the lock in DB
	_, err = store.db.Exec(`UPDATE file_locks SET expires_at = datetime('now', '-1 minute') WHERE project_id = ? AND path = ?`, projectID, path)
	if err != nil {
		t.Fatalf("Failed to force expire lock: %v", err)
	}

	// 3. Agent 2 attempts to acquire (should hijack)
	success, err = store.AcquireLock(projectID, path, agent2, 1*time.Second)
	if !success || err != nil {
		t.Fatalf("Agent2 failed to acquire/hijack expired lock: %v", err)
	}

	// 4. Verify ownership
	locks, _ := store.GetActiveLocks(projectID)
	if len(locks) != 1 || locks[0].AgentID != agent2 {
		t.Errorf("Lock should belong to agent2, got %v", locks)
	}
}
