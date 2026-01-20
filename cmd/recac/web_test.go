package main

import (
	"os"
	"path/filepath"
	"recac/internal/db"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebCommand_Run(t *testing.T) {
	// Setup temporary workspace and DB
	tmpDir := t.TempDir()

	// Create a dummy .recac.db
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, err := db.NewSQLiteStore(dbPath)
	require.NoError(t, err)

	// Seed some data
	featuresJSON := `{
		"project_name": "test-project",
		"features": [
			{"id": "task-1", "description": "Do something", "status": "pending", "priority": "high", "dependencies": {"depends_on_ids": []}},
			{"id": "task-2", "description": "Do something else", "status": "done", "priority": "low", "dependencies": {"depends_on_ids": ["task-1"]}}
		]
	}`
	err = store.SaveFeatures("default", featuresJSON)
	require.NoError(t, err)
	store.Close()

	// Switch to tmpDir
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// Check the command structure
	assert.Equal(t, "web [SESSION_NAME]", webCmd.Use)
	assert.NotNil(t, webCmd.RunE)
}

// Integration test for the Server logic
func TestWebServer_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, err := db.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	// Seed data
	featuresJSON := `{
		"project_name": "test-project",
		"features": [
			{"id": "A", "description": "Task A", "status": "done", "priority": "P0", "dependencies": {}},
			{"id": "B", "description": "Task B", "status": "pending", "priority": "P1", "dependencies": {"depends_on_ids": ["A"]}}
		]
	}`
	require.NoError(t, store.SaveFeatures("default", featuresJSON))

	// Note: We can't fully integration test the webCmd.RunE because it calls http.ListenAndServe which blocks.
	// The core logic is tested in internal/web/server_test.go
}
