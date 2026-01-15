package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB is a helper to create a new DB for each test, ensuring isolation.
func setupPostgresTestDB(t *testing.T) *PostgresStore {
	t.Helper()

	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable"
	}

	store, err := NewPostgresStore(dsn)
	require.NoError(t, err, "Failed to create store")

	// Truncate tables to ensure clean state for each test
	tables := []string{"observations", "signals", "project_features", "project_specs", "file_locks"}
	for _, table := range tables {
		_, err := store.db.Exec(fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table))
		require.NoError(t, err, "Failed to truncate table %s", table)
	}

	t.Cleanup(func() {
		store.Close()
	})

	return store
}

func TestPostgresStore_Observations(t *testing.T) {
	store := setupPostgresTestDB(t)
	projectID := "test-project"
	agentID := "test-agent"

	// Test 1: SaveObservation
	content1 := "Observed a test event"
	err := store.SaveObservation(projectID, agentID, content1)
	require.NoError(t, err, "SaveObservation failed")

	// Test 2: QueryHistory
	history, err := store.QueryHistory(projectID, 10)
	require.NoError(t, err, "QueryHistory failed")
	require.Len(t, history, 1, "Expected 1 observation")
	assert.Equal(t, agentID, history[0].AgentID)
	assert.Equal(t, content1, history[0].Content)

	// Test 3: Multiple Insertions and Order
	content2 := "Second event"
	err = store.SaveObservation(projectID, agentID, content2)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	content3 := "Third event"
	err = store.SaveObservation(projectID, agentID, content3)
	require.NoError(t, err)

	history, err = store.QueryHistory(projectID, 2)
	require.NoError(t, err, "QueryHistory with limit failed")
	require.Len(t, history, 2, "Expected 2 observations")

	assert.Equal(t, content3, history[0].Content)
	assert.Equal(t, content2, history[1].Content)

	// Test 4: Query history for a different project
	history, err = store.QueryHistory("other-project", 10)
	require.NoError(t, err)
	assert.Empty(t, history, "Should not get history for another project")
}

func TestPostgresStore_Signals(t *testing.T) {
	store := setupPostgresTestDB(t)
	projectID := "sig-project"

	val, err := store.GetSignal(projectID, "non-existent")
	require.NoError(t, err)
	assert.Equal(t, "", val, "Expected empty string for non-existent signal")

	key, value := "STATUS", "IN_PROGRESS"
	err = store.SetSignal(projectID, key, value)
	require.NoError(t, err)

	retrievedVal, err := store.GetSignal(projectID, key)
	require.NoError(t, err)
	assert.Equal(t, value, retrievedVal)

	newValue := "COMPLETED"
	err = store.SetSignal(projectID, key, newValue)
	require.NoError(t, err)

	retrievedVal, err = store.GetSignal(projectID, key)
	require.NoError(t, err)
	assert.Equal(t, newValue, retrievedVal)

	err = store.DeleteSignal(projectID, key)
	require.NoError(t, err)

	deletedVal, err := store.GetSignal(projectID, key)
	require.NoError(t, err)
	assert.Equal(t, "", deletedVal, "Expected empty string for deleted signal")
}

func TestPostgresStore_FeaturesAndSpecs(t *testing.T) {
	store := setupPostgresTestDB(t)
	projectID := "feat-project"

	specContent := "This is the project spec."
	err := store.SaveSpec(projectID, specContent)
	require.NoError(t, err)

	retrievedSpec, err := store.GetSpec(projectID)
	require.NoError(t, err)
	assert.Equal(t, specContent, retrievedSpec)

	featureList := FeatureList{
		ProjectName: "Test Project",
		Features: []Feature{
			{ID: "F01", Status: "pending", Passes: false},
			{ID: "F02", Status: "pending", Passes: false},
		},
	}
	featureBytes, err := json.Marshal(featureList)
	require.NoError(t, err)
	featuresContent := string(featureBytes)

	err = store.SaveFeatures(projectID, featuresContent)
	require.NoError(t, err)

	retrievedFeatures, err := store.GetFeatures(projectID)
	require.NoError(t, err)
	assert.JSONEq(t, featuresContent, retrievedFeatures)

	err = store.UpdateFeatureStatus(projectID, "F01", "done", true)
	require.NoError(t, err)

	updatedFeaturesStr, err := store.GetFeatures(projectID)
	require.NoError(t, err)
	var updatedFeatureList FeatureList
	err = json.Unmarshal([]byte(updatedFeaturesStr), &updatedFeatureList)
	require.NoError(t, err)

	assert.Equal(t, "done", updatedFeatureList.Features[0].Status)
	assert.True(t, updatedFeatureList.Features[0].Passes)
	assert.Equal(t, "pending", updatedFeatureList.Features[1].Status)

	err = store.UpdateFeatureStatus(projectID, "NON_EXISTENT_ID", "done", true)
	assert.Error(t, err, "Should error when feature ID is not found")

	err = store.UpdateFeatureStatus("other-project", "F01", "done", true)
	assert.Error(t, err, "Should error when no features are saved for project")
	assert.Equal(t, sql.ErrNoRows, err)
}

func TestPostgresStore_Locks(t *testing.T) {
	store := setupPostgresTestDB(t)
	projectID := "lock-project"
	path1 := "/file/a"
	path2 := "/file/b"
	agent1 := "agent-1"
	agent2 := "agent-2"

	locked, err := store.AcquireLock(projectID, path1, agent1, time.Second)
	require.NoError(t, err)
	assert.True(t, locked, "agent1 should acquire lock")

	locked, err = store.AcquireLock(projectID, path1, agent2, 100*time.Millisecond)
	require.NoError(t, err)
	assert.False(t, locked, "agent2 should fail to acquire lock")

	locked, err = store.AcquireLock(projectID, path1, agent1, time.Second)
	require.NoError(t, err)
	assert.True(t, locked, "agent1 should renew its lock")

	locked, err = store.AcquireLock(projectID, path2, agent2, time.Second)
	require.NoError(t, err)
	assert.True(t, locked, "agent2 should acquire lock on different path")

	activeLocks, err := store.GetActiveLocks(projectID)
	require.NoError(t, err)
	assert.Len(t, activeLocks, 2)
	foundPaths := []string{activeLocks[0].Path, activeLocks[1].Path}
	assert.Contains(t, foundPaths, path1)
	assert.Contains(t, foundPaths, path2)

	err = store.ReleaseLock(projectID, path1, agent1)
	require.NoError(t, err)

	locked, err = store.AcquireLock(projectID, path1, agent2, time.Second)
	require.NoError(t, err)
	assert.True(t, locked, "agent2 should now acquire lock on path1")

	err = store.ReleaseAllLocks(projectID, agent2)
	require.NoError(t, err)

	activeLocks, err = store.GetActiveLocks(projectID)
	require.NoError(t, err)
	assert.Empty(t, activeLocks, "all of agent2's locks should be gone")

	_, _ = store.AcquireLock(projectID, path1, agent1, time.Second)
	err = store.ReleaseLock(projectID, path1, "MANAGER")
	require.NoError(t, err)
	activeLocks, err = store.GetActiveLocks(projectID)
	require.NoError(t, err)
	assert.Empty(t, activeLocks, "manager should have released the lock")
}

func TestPostgresStore_Cleanup(t *testing.T) {
	store := setupPostgresTestDB(t)
	projectID := "cleanup-project"

	_, err := store.db.Exec(`INSERT INTO file_locks (project_id, path, agent_id, expires_at) VALUES ($1, $2, $3, $4)`,
		projectID, "/file/expired", "agent-expire", time.Now().Add(-1*time.Hour))
	require.NoError(t, err)

	oldTime := time.Now().Add(-48 * time.Hour)
	_, err = store.db.Exec(`INSERT INTO signals (project_id, key, value, created_at) VALUES ($1, $2, $3, $4)`,
		projectID, "OLD_SIGNAL", "data", oldTime)
	require.NoError(t, err)

	_, err = store.db.Exec(`INSERT INTO signals (project_id, key, value, created_at) VALUES ($1, $2, $3, $4)`,
		projectID, "QA_PASSED", "true", oldTime)
	require.NoError(t, err)

	err = store.Cleanup()
	require.NoError(t, err)

	var count int
	err = store.db.QueryRow(`SELECT COUNT(*) FROM file_locks WHERE project_id = $1`, projectID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Expired lock should have been cleaned up")

	val, err := store.GetSignal(projectID, "OLD_SIGNAL")
	require.NoError(t, err)
	assert.Equal(t, "", val, "Old non-critical signal should be cleaned up")

	val, err = store.GetSignal(projectID, "QA_PASSED")
	require.NoError(t, err)
	assert.Equal(t, "true", val, "Old critical signal should not be cleaned up")
}

func TestPostgresStore_ConnectionError(t *testing.T) {
	t.Run("Invalid DSN", func(t *testing.T) {
		_, err := NewPostgresStore("postgres://invalid:user@localhost/db")
		assert.Error(t, err, "Expected connection error")
	})
}
