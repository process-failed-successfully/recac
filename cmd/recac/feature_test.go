package main

import (
	"bytes"
	"encoding/json"
	"os"
	"recac/internal/db"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureCommands(t *testing.T) {
	// Setup temp directory
	tempDir, err := os.MkdirTemp("", "recac-feature-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Switch to temp dir so the command runs there
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Create an empty feature list to start
	initialList := db.FeatureList{
		ProjectName: "test-project",
		Features:    []db.Feature{},
	}
	data, _ := json.Marshal(initialList)
	os.WriteFile("feature_list.json", data, 0644)

	t.Run("Add Task", func(t *testing.T) {
		cmd := featureAddCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		// Execute add
		err := cmd.RunE(cmd, []string{"Implement login feature"})
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "Added task: task-1")

		// Verify file
		content, _ := os.ReadFile("feature_list.json")
		var fl db.FeatureList
		json.Unmarshal(content, &fl)
		require.Len(t, fl.Features, 1)
		assert.Equal(t, "Implement login feature", fl.Features[0].Description)
		assert.Equal(t, "task-1", fl.Features[0].ID)
		assert.Equal(t, "todo", fl.Features[0].Status)
	})

	t.Run("List Tasks", func(t *testing.T) {
		cmd := featureListCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := cmd.RunE(cmd, []string{})
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "ID")
		assert.Contains(t, output, "STATUS")
		assert.Contains(t, output, "task-1")
		assert.Contains(t, output, "Implement login feature")
	})

	t.Run("Complete Task", func(t *testing.T) {
		cmd := featureDoneCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := cmd.RunE(cmd, []string{"task-1"})
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "Updated task task-1 to done")

		// Verify file
		content, _ := os.ReadFile("feature_list.json")
		var fl db.FeatureList
		json.Unmarshal(content, &fl)
		assert.Equal(t, "done", fl.Features[0].Status)
		assert.True(t, fl.Features[0].Passes)
	})

	t.Run("Pending Task", func(t *testing.T) {
		cmd := featurePendingCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := cmd.RunE(cmd, []string{"task-1"})
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "Updated task task-1 to todo")

		// Verify file
		content, _ := os.ReadFile("feature_list.json")
		var fl db.FeatureList
		json.Unmarshal(content, &fl)
		assert.Equal(t, "todo", fl.Features[0].Status)
		assert.False(t, fl.Features[0].Passes)
	})

	t.Run("Add Second Task", func(t *testing.T) {
		cmd := featureAddCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := cmd.RunE(cmd, []string{"Setup database"})
		require.NoError(t, err)

		content, _ := os.ReadFile("feature_list.json")
		var fl db.FeatureList
		json.Unmarshal(content, &fl)
		require.Len(t, fl.Features, 2)
		assert.Equal(t, "task-2", fl.Features[1].ID)
	})
}
