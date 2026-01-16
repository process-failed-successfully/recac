package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphCmd(t *testing.T) {
	// 1. Setup Workspace and DB
	tmpDir, err := os.MkdirTemp("", "recac-graph-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, err := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	require.NoError(t, err)

	projectName := "test-project"

	// 2. Insert Features
	features := []db.Feature{
		{
			ID:          "task-1",
			Description: "Task 1",
			Status:      "done",
			Passes:      true,
		},
		{
			ID:          "task-2",
			Description: "Task 2",
			Status:      "in_progress",
			Dependencies: db.FeatureDependencies{
				DependsOnIDs: []string{"task-1"},
			},
		},
		{
			ID:          "task-3",
			Description: "Task 3",
			Status:      "todo",
			Dependencies: db.FeatureDependencies{
				DependsOnIDs: []string{"task-2"},
			},
		},
	}
	fl := db.FeatureList{Features: features}
	flBytes, _ := json.Marshal(fl)
	err = store.SaveFeatures(projectName, string(flBytes))
	require.NoError(t, err)
	store.Close()

	// 3. Setup Mock Session Manager
	mockSM := NewMockSessionManager()
	session := &runner.SessionState{
		Name:      projectName,
		Workspace: tmpDir,
		Status:    "completed",
		StartTime: time.Now(),
	}
	mockSM.Sessions[projectName] = session

	// Override factory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// 4. Run Command with isolated root
	cmd := newGraphCmd()
	root := &cobra.Command{Use: "recac"}
	root.AddCommand(cmd)

	output, err := executeCommand(root, "graph", projectName)
	require.NoError(t, err)

	// 5. Verify Output
	assert.Contains(t, output, "graph TD")
	assert.Contains(t, output, `task_1["Task 1"]:::done`)
	assert.Contains(t, output, `task_2["Task 2"]:::inprogress`)
	assert.Contains(t, output, `task_3["Task 3"]:::pending`)
	assert.Contains(t, output, "task_1 --> task_2")
	assert.Contains(t, output, "task_2 --> task_3")

	// Legend
	assert.Contains(t, output, "classDef done")
	assert.Contains(t, output, "classDef inprogress")
}

func TestGraphCmd_NoFeatures(t *testing.T) {
	// 1. Setup Workspace and DB (empty)
	tmpDir, err := os.MkdirTemp("", "recac-graph-test-empty-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, ".recac.db")
	_, err = db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	require.NoError(t, err)

	projectName := "empty-project"

	// 2. Setup Mock Session Manager
	mockSM := NewMockSessionManager()
	session := &runner.SessionState{
		Name:      projectName,
		Workspace: tmpDir,
		Status:    "completed",
		StartTime: time.Now(),
	}
	mockSM.Sessions[projectName] = session

	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// 3. Run Command (expect error)
	cmd := newGraphCmd()
	root := &cobra.Command{Use: "recac"}
	root.AddCommand(cmd)

	_, err = executeCommand(root, "graph", projectName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no features found")
}

func TestGraphCmd_LatestSession(t *testing.T) {
	// 1. Setup Workspace and DB
	tmpDir, err := os.MkdirTemp("", "recac-graph-test-latest-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, err := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	require.NoError(t, err)

	projectName := "latest-project"
	features := []db.Feature{{ID: "task-A", Description: "A", Status: "done"}}
	fl := db.FeatureList{Features: features}
	flBytes, _ := json.Marshal(fl)
	store.SaveFeatures(projectName, string(flBytes))
	store.Close()

	// 2. Setup Mock Session Manager with multiple sessions
	mockSM := NewMockSessionManager()

	// Old session
	mockSM.Sessions["old-session"] = &runner.SessionState{
		Name:      "old-session",
		Workspace: "/tmp/nowhere", // Should not be accessed
		StartTime: time.Now().Add(-2 * time.Hour),
	}

	// New session
	mockSM.Sessions[projectName] = &runner.SessionState{
		Name:      projectName,
		Workspace: tmpDir,
		StartTime: time.Now(),
	}

	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// 3. Run Command without args
	cmd := newGraphCmd()
	root := &cobra.Command{Use: "recac"}
	root.AddCommand(cmd)

	output, err := executeCommand(root, "graph")
	require.NoError(t, err)

	// 4. Verify Output (should use the latest session)
	assert.Contains(t, output, `task_A["A"]:::done`)
}
