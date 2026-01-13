package db

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestStore(t *testing.T) *SQLiteStore {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err, "Failed to create store")
	t.Cleanup(func() {
		store.Close()
	})
	return store
}

func TestSQLiteStore_Features(t *testing.T) {
	store := setupTestStore(t)
	projectID := "test-project"

	// 1. GetFeatures on empty DB
	features, err := store.GetFeatures(projectID)
	require.NoError(t, err)
	assert.Equal(t, "", features)

	// 2. SaveFeatures
	initialFeatures := `{"features":[{"id":"feat-1","status":"pending","passes":false}]}`
	err = store.SaveFeatures(projectID, initialFeatures)
	require.NoError(t, err)

	// 3. GetFeatures after save
	features, err = store.GetFeatures(projectID)
	require.NoError(t, err)
	assert.Equal(t, initialFeatures, features)

	// 4. UpdateFeatureStatus - Success
	err = store.UpdateFeatureStatus(projectID, "feat-1", "completed", true)
	require.NoError(t, err)

	// 5. Verify update
	features, err = store.GetFeatures(projectID)
	require.NoError(t, err)

	var fl FeatureList
	err = json.Unmarshal([]byte(features), &fl)
	require.NoError(t, err)
	require.Len(t, fl.Features, 1)
	assert.Equal(t, "completed", fl.Features[0].Status)
	assert.True(t, fl.Features[0].Passes)
}

func TestSQLiteStore_UpdateFeatureStatus_Errors(t *testing.T) {
	store := setupTestStore(t)
	projectID := "test-project"

	// 1. Error on non-existent project
	err := store.UpdateFeatureStatus(projectID, "feat-1", "completed", true)
	assert.EqualError(t, err, "no features found in DB for project test-project")

	// 2. Error on non-existent feature ID
	initialFeatures := `{"features":[{"id":"feat-1","status":"pending","passes":false}]}`
	err = store.SaveFeatures(projectID, initialFeatures)
	require.NoError(t, err)
	err = store.UpdateFeatureStatus(projectID, "feat-2", "completed", true)
	assert.EqualError(t, err, "feature ID feat-2 not found")

	// 3. Error on invalid JSON
	err = store.SaveFeatures(projectID, `{"features":...`)
	require.NoError(t, err)
	err = store.UpdateFeatureStatus(projectID, "feat-1", "completed", true)
	assert.ErrorContains(t, err, "failed to unmarshal features")
}

func TestSQLiteStore_Specs(t *testing.T) {
	store := setupTestStore(t)
	projectID := "test-project"

	// 1. GetSpec on empty DB
	spec, err := store.GetSpec(projectID)
	require.NoError(t, err)
	assert.Equal(t, "", spec)

	// 2. SaveSpec
	initialSpec := "This is a test spec."
	err = store.SaveSpec(projectID, initialSpec)
	require.NoError(t, err)

	// 3. GetSpec after save
	spec, err = store.GetSpec(projectID)
	require.NoError(t, err)
	assert.Equal(t, initialSpec, spec)
}
