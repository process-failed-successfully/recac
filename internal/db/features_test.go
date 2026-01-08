package db

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteFeatures(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_features.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 1. Test SaveFeatures & GetFeatures
	projectID := "test-project"
	fl := FeatureList{
		ProjectName: projectID,
		Features: []Feature{
			{ID: "f1", Description: "Feature 1", Status: "pending"},
		},
	}
	data, _ := json.Marshal(fl)
	featuresJson := string(data)

	if err := store.SaveFeatures(projectID, featuresJson); err != nil {
		t.Errorf("SaveFeatures failed: %v", err)
	}

	retrieved, err := store.GetFeatures(projectID)
	if err != nil {
		t.Errorf("GetFeatures failed: %v", err)
	}
	if retrieved != featuresJson {
		t.Errorf("Expected features %s, got %s", featuresJson, retrieved)
	}

	// 2. Test UpdateFeatureStatus
	if err := store.UpdateFeatureStatus(projectID, "f1", "done", true); err != nil {
		t.Errorf("UpdateFeatureStatus failed: %v", err)
	}

	retrieved, _ = store.GetFeatures(projectID)
	var retrievedFL FeatureList
	json.Unmarshal([]byte(retrieved), &retrievedFL)
	if retrievedFL.Features[0].Status != "done" {
		t.Errorf("Expected status 'done', got '%s'", retrievedFL.Features[0].Status)
	}
	if !retrievedFL.Features[0].Passes {
		t.Error("Expected passes to be true")
	}

	// 3. Test UpdateFeatureStatus (Non-existent)
	if err := store.UpdateFeatureStatus(projectID, "non-existent", "done", true); err == nil {
		t.Error("Expected error for non-existent feature")
	}
}

func TestSQLiteLocks(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_locks.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	path := "file.txt"
	agentID := "agent-1"
	projectID := "test-project"

	// 1. Acquire Lock
	acquired, err := store.AcquireLock(projectID, path, agentID, 100*time.Millisecond)
	if err != nil {
		t.Errorf("AcquireLock failed: %v", err)
	}
	if !acquired {
		t.Error("Expected to acquire lock")
	}

	// 2. Acquire Duplicate Lock (Same Agent) -> Should extend/succeed
	acquired, err = store.AcquireLock(projectID, path, agentID, 100*time.Millisecond)
	if err != nil {
		t.Errorf("AcquireLock (renew) failed: %v", err)
	}
	if !acquired {
		t.Error("Expected to renew lock")
	}

	// 3. Acquire Lock (Different Agent) -> Should fail
	acquired, err = store.AcquireLock(projectID, path, "agent-2", 100*time.Millisecond)
	if err != nil {
		t.Errorf("AcquireLock (contention) failed: %v", err)
	}
	if acquired {
		t.Error("Expected fail to acquire lock held by another agent")
	}

	// 4. GetActiveLocks
	locks, err := store.GetActiveLocks(projectID)
	if err != nil {
		t.Fatalf("GetActiveLocks failed: %v", err)
	}
	if len(locks) != 1 {
		t.Errorf("Expected 1 lock, got %d", len(locks))
	}
	if locks[0].Path != path {
		t.Errorf("Expected lock path %s, got %s", path, locks[0].Path)
	}

	// 5. Release Lock
	if err := store.ReleaseLock(projectID, path, agentID); err != nil {
		t.Errorf("ReleaseLock failed: %v", err)
	}

	// 6. Acquire Lock (Different Agent) -> Should succeed now
	acquired, err = store.AcquireLock(projectID, path, "agent-2", 100*time.Millisecond)
	if err != nil {
		t.Errorf("AcquireLock (after release) failed: %v", err)
	}
	if !acquired {
		t.Error("Expected to acquire lock after release")
	}

	// 7. Release All Locks
	if err := store.ReleaseAllLocks(projectID, "agent-2"); err != nil {
		t.Errorf("ReleaseAllLocks failed: %v", err)
	}
	// 8. Final check - no locks
	locks, _ = store.GetActiveLocks(projectID)
	if len(locks) > 0 {
		t.Errorf("Expected 0 locks after ReleaseAllLocks, got %d", len(locks))
	}
}
