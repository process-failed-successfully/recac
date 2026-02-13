package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/db"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a temporary SQLite database for testing
func setupTestDB(t *testing.T) (db.Store, string) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := db.StoreConfig{
		Type:             "sqlite",
		ConnectionString: dbPath,
	}

	store, err := db.NewStore(config)
	require.NoError(t, err)

	return store, dbPath
}

func TestFeatureCommands(t *testing.T) {
	store, dbPath := setupTestDB(t)
	defer store.Close()

	// Override getStoreFunc
	originalGetStore := getStoreFunc
	defer func() { getStoreFunc = originalGetStore }()

	getStoreFunc = func() (db.Store, error) {
		config := db.StoreConfig{
			Type:             "sqlite",
			ConnectionString: dbPath,
		}
		return db.NewStore(config)
	}

	// Set project ID
	viper.Set("project_id", "test-project")

	// Prepare sample data
	features := db.FeatureList{
		ProjectName: "Test Project",
		Features: []db.Feature{
			{ID: "F-1", Status: "Todo", Passes: false, Description: "Feature 1"},
			{ID: "F-2", Status: "Done", Passes: true, Description: "Feature 2"},
		},
	}
	data, err := json.Marshal(features)
	require.NoError(t, err)

	tmpFile := filepath.Join(t.TempDir(), "features.json")
	err = os.WriteFile(tmpFile, data, 0644)
	require.NoError(t, err)

	t.Log("Testing Import...")
	// Test Import
	// We execute via rootCmd to ensure correct command routing
	rootCmd.SetArgs([]string{"feature", "import", tmpFile})
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err = rootCmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Successfully imported 2 features")
	t.Log("Import success")

	// Verify DB content
	content, err := store.GetFeatures("test-project")
	require.NoError(t, err)
	require.Contains(t, content, "Feature 1")
	t.Log("DB verify success")

	// Test List
	buf.Reset()
	rootCmd.SetArgs([]string{"feature", "list"})
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err = rootCmd.Execute()
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "F-1")
	assert.Contains(t, output, "Todo")
	assert.Contains(t, output, "F-2")
	assert.Contains(t, output, "Done")

	// Test Status Update
	buf.Reset()
	rootCmd.SetArgs([]string{"feature", "status", "F-1", "In Progress", "true"})
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err = rootCmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "updated: status=In Progress, passes=true")

	// Verify update in DB
	content, err = store.GetFeatures("test-project")
	require.NoError(t, err)
	var fl db.FeatureList
	err = json.Unmarshal([]byte(content), &fl)
	require.NoError(t, err)

	found := false
	for _, f := range fl.Features {
		if f.ID == "F-1" {
			assert.Equal(t, "In Progress", f.Status)
			assert.True(t, f.Passes)
			found = true
			break
		}
	}
	assert.True(t, found)
}
