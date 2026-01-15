package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupTestDB is a helper to initialize a database for testing.
// It returns a ready-to-use Store and a cleanup function.
func setupTestDB(t *testing.T, dbType string) (Store, func()) {
	t.Helper()

	var store Store
	var err error
	var cleanup func() = func() {}

	switch dbType {
	case "sqlite":
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")
		store, err = NewSQLiteStore(dbPath)
		if err != nil {
			t.Fatalf("Failed to create SQLite store: %v", err)
		}
		cleanup = func() {
			store.Close()
		}
	case "postgres":
		dsn := os.Getenv("PG_TEST_DSN")
		if dsn == "" {
			t.Skip("Skipping postgres tests: PG_TEST_DSN not set")
		}
		store, err = NewPostgresStore(dsn)
		if err != nil {
			t.Fatalf("Failed to create Postgres store: %v", err)
		}

		pgStore := store.(*PostgresStore)
		// Truncate tables for a clean slate
		tables := []string{"observations", "signals", "project_features", "project_specs", "file_locks"}
		for _, table := range tables {
			_, err := pgStore.db.Exec("TRUNCATE " + table + " RESTART IDENTITY CASCADE")
			if err != nil {
				t.Fatalf("Failed to truncate table %s: %v", table, err)
			}
		}

		cleanup = func() {
			store.Close()
		}
	default:
		t.Fatalf("Unsupported dbType: %s", dbType)
	}

	return store, cleanup
}

func TestNewStore(t *testing.T) {
	// To test the postgres happy path, we need a DSN
	dsn := os.Getenv("PG_TEST_DSN")
	if dsn == "" {
		// If no DSN is available, we can't test the postgres creation path.
		// We'll log this, but the tests for postgres functionality will be skipped anyway.
		t.Log("PG_TEST_DSN not set, skipping postgres creation test case in TestNewStore")
	}

	// Create a temporary directory for sqlite paths
	tmpDir := t.TempDir()
	sqlitePath := filepath.Join(tmpDir, "test_factory.db")

	testCases := []struct {
		name         string
		config       StoreConfig
		expectError  bool
		expectedType interface{}
	}{
		{
			name: "SQLite with specific path",
			config: StoreConfig{
				Type:             "sqlite",
				ConnectionString: sqlitePath,
			},
			expectError:  false,
			expectedType: &SQLiteStore{},
		},
		{
			name: "SQLite with default path",
			config: StoreConfig{
				Type: "sqlite",
			},
			expectError:  false,
			expectedType: &SQLiteStore{},
		},
		{
			name: "Default to SQLite with empty type and path",
			config: StoreConfig{
				Type: "",
			},
			expectError:  false,
			expectedType: &SQLiteStore{},
		},
		{
			name: "Postgres with DSN",
			config: StoreConfig{
				Type:             "postgres",
				ConnectionString: dsn,
			},
			expectError:  dsn == "", // Expect error only if DSN is not set
			expectedType: &PostgresStore{},
		},
		{
			name: "Postgres with empty DSN",
			config: StoreConfig{
				Type: "postgres",
			},
			expectError:  true,
			expectedType: nil,
		},
		{
			name: "Unsupported store type",
			config: StoreConfig{
				Type: "mongo",
			},
			expectError:  true,
			expectedType: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Skip the postgres happy path if DSN is not set
			if tc.name == "Postgres with DSN" && dsn == "" {
				t.Skip("Skipping postgres creation test: PG_TEST_DSN not set")
			}

			store, err := NewStore(tc.config)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected an error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Did not expect an error but got: %v", err)
				}
				if store == nil {
					t.Errorf("Expected a store instance but got nil")
					return
				}
				defer store.Close()

				// Check type
				if _, ok := store.(*SQLiteStore); ok {
					if _, ok := tc.expectedType.(*SQLiteStore); !ok {
						t.Errorf("Expected type %T but got *SQLiteStore", tc.expectedType)
					}
				} else if _, ok := store.(*PostgresStore); ok {
					if _, ok := tc.expectedType.(*PostgresStore); !ok {
						t.Errorf("Expected type %T but got *PostgresStore", tc.expectedType)
					}
				}
			}
		})
	}
	// Cleanup the default .recac.db if it was created
	os.Remove(".recac.db")
}

// runStoreTests is a helper to run the same test suite against different database backends.
func runStoreTests(t *testing.T, dbType string) {
	store, cleanup := setupTestDB(t, dbType)
	defer cleanup()

	projectID := "test-project"
	agentID := "test-agent"

	// Test Observation methods
	t.Run("TestObservations", func(t *testing.T) {
		// 1. Initial query should be empty
		history, err := store.QueryHistory(projectID, 10)
		if err != nil {
			t.Fatalf("QueryHistory on empty db failed: %v", err)
		}
		if len(history) != 0 {
			t.Fatalf("Expected 0 observations, got %d", len(history))
		}

		// 2. Save an observation
		content1 := "First observation"
		if err := store.SaveObservation(projectID, agentID, content1); err != nil {
			t.Fatalf("SaveObservation failed: %v", err)
		}

		// 3. Query again
		history, err = store.QueryHistory(projectID, 10)
		if err != nil {
			t.Fatalf("QueryHistory after save failed: %v", err)
		}
		if len(history) != 1 {
			t.Fatalf("Expected 1 observation, got %d", len(history))
		}
		if history[0].Content != content1 || history[0].AgentID != agentID {
			t.Errorf("Unexpected observation content or agentID")
		}

		// 4. Test limit and order
		content2 := "Second observation"
		// Ensure timestamps are distinct
		time.Sleep(5 * time.Millisecond)
		store.SaveObservation(projectID, agentID, content2)

		history, err = store.QueryHistory(projectID, 1)
		if err != nil {
			t.Fatalf("QueryHistory with limit failed: %v", err)
		}
		if len(history) != 1 {
			t.Fatalf("Expected 1 observation with limit, got %d", len(history))
		}
		if history[0].Content != content2 {
			t.Errorf("Expected newest observation first, but got %s", history[0].Content)
		}
	})

	// Test Signal methods
	t.Run("TestSignals", func(t *testing.T) {
		key := "test-signal"
		value := "test-value"

		// 1. Get non-existent signal
		val, err := store.GetSignal(projectID, key)
		if err != nil {
			t.Fatalf("GetSignal for non-existent key failed: %v", err)
		}
		if val != "" {
			t.Fatalf("Expected empty string for non-existent key, got %s", val)
		}

		// 2. Set signal
		if err := store.SetSignal(projectID, key, value); err != nil {
			t.Fatalf("SetSignal failed: %v", err)
		}

		// 3. Get signal
		val, err = store.GetSignal(projectID, key)
		if err != nil {
			t.Fatalf("GetSignal failed: %v", err)
		}
		if val != value {
			t.Fatalf("Expected value %s, got %s", value, val)
		}

		// 4. Update signal
		newValue := "new-value"
		if err := store.SetSignal(projectID, key, newValue); err != nil {
			t.Fatalf("SetSignal (update) failed: %v", err)
		}
		val, err = store.GetSignal(projectID, key)
		if err != nil {
			t.Fatalf("GetSignal after update failed: %v", err)
		}
		if val != newValue {
			t.Fatalf("Expected updated value %s, got %s", newValue, val)
		}

		// 5. Delete signal
		if err := store.DeleteSignal(projectID, key); err != nil {
			t.Fatalf("DeleteSignal failed: %v", err)
		}

		// 6. Get deleted signal
		val, err = store.GetSignal(projectID, key)
		if err != nil {
			t.Fatalf("GetSignal for deleted key failed: %v", err)
		}
		if val != "" {
			t.Fatalf("Expected empty string for deleted key, got %s", val)
		}
	})

	// Test Features and Spec methods
	t.Run("TestFeaturesAndSpec", func(t *testing.T) {
		// 1. Features
		featuresJSON := `{"project_name":"Test","features":[{"id":"F1","description":"Feature 1","status":"pending"}]}`
		if err := store.SaveFeatures(projectID, featuresJSON); err != nil {
			t.Fatalf("SaveFeatures failed: %v", err)
		}
		retrieved, err := store.GetFeatures(projectID)
		if err != nil {
			t.Fatalf("GetFeatures failed: %v", err)
		}
		if retrieved != featuresJSON {
			t.Fatalf("Mismatched features content")
		}

		// 2. Spec
		specContent := "Application Specification v1"
		if err := store.SaveSpec(projectID, specContent); err != nil {
			t.Fatalf("SaveSpec failed: %v", err)
		}
		retrievedSpec, err := store.GetSpec(projectID)
		if err != nil {
			t.Fatalf("GetSpec failed: %v", err)
		}
		if retrievedSpec != specContent {
			t.Fatalf("Mismatched spec content")
		}

		// 3. Update Feature Status
		if err := store.UpdateFeatureStatus(projectID, "F1", "completed", true); err != nil {
			t.Fatalf("UpdateFeatureStatus failed: %v", err)
		}
		updatedFeatures, _ := store.GetFeatures(projectID)
		expectedUpdate := `{"project_name":"Test","features":[{"id":"F1","category":"","priority":"","description":"Feature 1","status":"completed","passes":true,"steps":null,"dependencies":{"depends_on_ids":null,"exclusive_write_paths":null,"read_only_paths":null}}]}`
		if updatedFeatures != expectedUpdate {
			t.Fatalf("UpdateFeatureStatus did not modify content as expected.\nGot: %s\nExp: %s", updatedFeatures, expectedUpdate)
		}

		// 4. Update non-existent feature
		err = store.UpdateFeatureStatus(projectID, "F2", "completed", true)
		if err == nil {
			t.Fatalf("Expected an error when updating a non-existent feature, but got none")
		}

		// 5. Update with invalid JSON
		// Save invalid JSON first
		store.SaveFeatures(projectID, `{"invalid_json": "test"`)
		err = store.UpdateFeatureStatus(projectID, "F1", "completed", true)
		if err == nil {
			t.Fatalf("Expected an error when updating with invalid JSON, but got none")
		}

		// 6. Get non-existent
		val, err := store.GetFeatures("non-existent")
		if err != nil {
			t.Fatalf("GetFeatures for non-existent project failed: %v", err)
		}
		if val != "" {
			t.Fatalf("Expected empty string for non-existent features, got %s", val)
		}
	})

	// Test Locking and Cleanup
	t.Run("TestLockingAndCleanup", func(t *testing.T) {
		path1 := "/file/one"
		path2 := "/file/two"
		otherAgent := "other-agent"

		// 1. Acquire lock
		acquired, err := store.AcquireLock(projectID, path1, agentID, time.Second)
		if err != nil {
			t.Fatalf("AcquireLock failed: %v", err)
		}
		if !acquired {
			t.Fatalf("Failed to acquire lock")
		}

		// 2. Try to acquire locked path (and fail due to timeout)
		acquired, err = store.AcquireLock(projectID, path1, otherAgent, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("AcquireLock on locked path failed: %v", err)
		}
		if acquired {
			t.Fatalf("Should not have acquired a lock that is already held")
		}

		// 3. Get active locks
		locks, err := store.GetActiveLocks(projectID)
		if err != nil {
			t.Fatalf("GetActiveLocks failed: %v", err)
		}
		if len(locks) != 1 {
			t.Fatalf("Expected 1 active lock, got %d", len(locks))
		}
		if locks[0].Path != path1 || locks[0].AgentID != agentID {
			t.Errorf("Mismatched lock data")
		}

		// 4. Renew lock
		time.Sleep(10 * time.Millisecond) // ensure time passes
		acquired, err = store.AcquireLock(projectID, path1, agentID, time.Second)
		if err != nil {
			t.Fatalf("Failed to renew lock: %v", err)
		}
		if !acquired {
			t.Fatalf("Should have been able to renew lock")
		}
		locks, _ = store.GetActiveLocks(projectID)
		if len(locks) != 1 {
			t.Fatalf("Expected 1 active lock after renew, got %d", len(locks))
		}
		// Could check if expiry increased, but that's implementation-dependent.

		// 5. Release lock
		if err := store.ReleaseLock(projectID, path1, agentID); err != nil {
			t.Fatalf("ReleaseLock failed: %v", err)
		}
		locks, _ = store.GetActiveLocks(projectID)
		if len(locks) != 0 {
			t.Fatalf("Expected 0 active locks after release, got %d", len(locks))
		}

		// 6. Test expired lock hijacking
		// Manually insert an expired lock
		var expiredTime time.Time
		switch s := store.(type) {
		case *SQLiteStore:
			// For SQLite, "-1 minute" is a valid time modifier.
			// We need to execute a query that creates a timestamp in the past.
			expiredTime = time.Now().Add(-1 * time.Minute)
			s.db.Exec(`INSERT INTO file_locks (project_id, path, agent_id, expires_at) VALUES (?, ?, ?, ?)`, projectID, path2, agentID, expiredTime)
		case *PostgresStore:
			// For Postgres, we can use INTERVAL.
			s.db.Exec(`INSERT INTO file_locks (project_id, path, agent_id, expires_at) VALUES ($1, $2, $3, NOW() - INTERVAL '1 minute')`, projectID, path2, agentID)
		}

		acquired, err = store.AcquireLock(projectID, path2, otherAgent, time.Second)
		if err != nil {
			t.Fatalf("AcquireLock on expired lock failed: %v", err)
		}
		if !acquired {
			t.Fatalf("Failed to acquire expired lock")
		}
		locks, _ = store.GetActiveLocks(projectID)
		if len(locks) != 1 || locks[0].AgentID != otherAgent {
			t.Fatalf("Lock should have been hijacked by otherAgent")
		}
		store.ReleaseLock(projectID, path2, otherAgent) // cleanup

		// 7. ReleaseAllLocks
		store.AcquireLock(projectID, path1, agentID, time.Second)
		store.AcquireLock(projectID, path2, agentID, time.Second)
		if err := store.ReleaseAllLocks(projectID, agentID); err != nil {
			t.Fatalf("ReleaseAllLocks failed: %v", err)
		}
		locks, _ = store.GetActiveLocks(projectID)
		if len(locks) != 0 {
			t.Fatalf("Expected 0 active locks after ReleaseAll, got %d", len(locks))
		}

		// 8. Test manager override for ReleaseLock
		store.AcquireLock(projectID, path1, agentID, time.Second)
		if err := store.ReleaseLock(projectID, path1, "MANAGER"); err != nil {
			t.Fatalf("Manager failed to release lock: %v", err)
		}
		locks, _ = store.GetActiveLocks(projectID)
		if len(locks) != 0 {
			t.Fatalf("Expected 0 locks after manager release, got %d", len(locks))
		}

		// 9. Cleanup
		// Manually add an expired lock, an old (but critical) signal, an old non-critical signal,
		// and an old observation to test the cleanup logic.
		switch s := store.(type) {
		case *SQLiteStore:
			s.db.Exec(`INSERT INTO file_locks (project_id, path, agent_id, expires_at) VALUES (?, ?, ?, ?)`, projectID, "/expired", agentID, time.Now().Add(-2*time.Minute))
			s.db.Exec(`INSERT INTO signals (project_id, key, value, created_at) VALUES (?, ?, ?, ?)`, projectID, "COMPLETED", "true", time.Now().Add(-25*time.Hour))
			s.db.Exec(`INSERT INTO signals (project_id, key, value, created_at) VALUES (?, ?, ?, ?)`, projectID, "old-signal", "value", time.Now().Add(-25*time.Hour))
			// For observations, we can't easily fake the timestamp, but we can trust the query logic.
		case *PostgresStore:
			s.db.Exec(`INSERT INTO file_locks (project_id, path, agent_id, expires_at) VALUES ($1, $2, $3, NOW() - INTERVAL '2 minute')`, projectID, "/expired", agentID)
			s.db.Exec(`INSERT INTO signals (project_id, key, value, created_at) VALUES ($1, $2, $3, NOW() - INTERVAL '25 hour')`, projectID, "COMPLETED", "true")
			s.db.Exec(`INSERT INTO signals (project_id, key, value, created_at) VALUES ($1, $2, $3, NOW() - INTERVAL '25 hour')`, projectID, "old-signal", "value")
		}

		if err := store.Cleanup(); err != nil {
			t.Fatalf("Cleanup failed: %v", err)
		}

		// Verify expired lock is gone
		locks, _ = store.GetActiveLocks(projectID)
		for _, lock := range locks {
			if lock.Path == "/expired" {
				t.Errorf("Expired lock was not cleaned up")
			}
		}

		// Verify old signal is gone, but critical one remains
		val, _ := store.GetSignal(projectID, "old-signal")
		if val != "" {
			t.Errorf("Old signal was not cleaned up")
		}
		val, _ = store.GetSignal(projectID, "COMPLETED")
		if val == "" {
			t.Errorf("Critical signal was incorrectly cleaned up")
		}
	})
}

func TestStore_SQLite(t *testing.T) {
	runStoreTests(t, "sqlite")
}

func TestStore_Postgres(t *testing.T) {
	runStoreTests(t, "postgres")
}
