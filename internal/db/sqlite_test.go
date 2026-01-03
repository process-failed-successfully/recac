package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Test 1: SaveObservation
	agentID := "test-agent"
	content := "Observed a test event"
	if err := store.SaveObservation(agentID, content); err != nil {
		t.Errorf("SaveObservation failed: %v", err)
	}

	// Test 2: QueryHistory
	history, err := store.QueryHistory(10)
	if err != nil {
		t.Errorf("QueryHistory failed: %v", err)
	}

	if len(history) != 1 {
		t.Errorf("Expected 1 observation, got %d", len(history))
	}

	if history[0].AgentID != agentID {
		t.Errorf("Expected agentID %s, got %s", agentID, history[0].AgentID)
	}
	if history[0].Content != content {
		t.Errorf("Expected content %s, got %s", content, history[0].Content)
	}

	// Test 3: Multiple Insertions and Order
	store.SaveObservation(agentID, "Second event")
	time.Sleep(10 * time.Millisecond) // Ensure timestamp difference
	store.SaveObservation(agentID, "Third event")

	history, err = store.QueryHistory(2)
	if err != nil {
		t.Errorf("QueryHistory failed: %v", err)
	}

	if len(history) != 2 {
		t.Errorf("Expected 2 observations, got %d", len(history))
	}

	// Should be DESC order (newest first)
	if history[0].Content != "Third event" {
		t.Errorf("Expected newest event first, got %s", history[0].Content)
	}
}

func TestSQLiteStore_Errors(t *testing.T) {
	t.Run("Invalid Path", func(t *testing.T) {
		// Providing a directory path where a file is expected might cause issues for some drivers,
		// but for modernc.org/sqlite, let's try an empty path or a path that's a directory.
		tmpDir := t.TempDir()
		_, err := NewSQLiteStore(tmpDir)
		if err == nil {
			t.Error("Expected error for directory path")
		}
	})
}

func TestSQLiteStore_ReleaseLock(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, _ := NewSQLiteStore(dbPath)
	defer store.Close()

	path := "/test/path"
	agentID := "agent-1"

	// 1. Acquire lock
	store.AcquireLock(path, agentID, time.Second)

	// 2. Release by correct agent
	if err := store.ReleaseLock(path, agentID); err != nil {
		t.Errorf("ReleaseLock failed for owner: %v", err)
	}

	// 3. Re-acquire and release by MANAGER
	store.AcquireLock(path, agentID, time.Second)
	if err := store.ReleaseLock(path, "MANAGER"); err != nil {
		t.Errorf("ReleaseLock failed for MANAGER: %v", err)
	}

	// 4. Release non-existent lock (should not error)
	if err := store.ReleaseLock("non-existent", agentID); err != nil {
		t.Errorf("ReleaseLock for non-existent path failed: %v", err)
	}
}
