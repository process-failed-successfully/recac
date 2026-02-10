package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/orchestrator"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runTaskCommand executes the task command via the root command to ensure correct routing and flag parsing.
func runTaskCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	t.Logf("Running task command with args: %v", args)

	// Reset flags on all commands to ensure clean state
	resetTaskFlags(rootCmd)

	// Construct full args list: "task" + args
	fullArgs := append([]string{"task"}, args...)
	rootCmd.SetArgs(fullArgs)

	// Capture output
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)

	// Execute via root to allow Cobra to route to subcommands (task -> add/list)
	err := rootCmd.Execute()

	// If there's output on stderr, log it for debugging
	if errBuf.Len() > 0 {
		t.Logf("Stderr: %s", errBuf.String())
	}

	return outBuf.String(), err
}

func resetTaskFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			f.Value.Set(f.DefValue)
			f.Changed = false
		}
	})
	for _, c := range cmd.Commands() {
		resetTaskFlags(c)
	}
}

func TestTaskAdd_FileDir(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()

	// Reset viper
	viper.Reset()
	defer viper.Reset()

	// Test adding a task
	out, err := runTaskCommand(t, "add", "Fix bug in login", "--poller", "file-dir", "--watch-dir", tmpDir, "--repo", "https://github.com/example/repo")
	require.NoError(t, err)
	assert.Contains(t, out, "Task added")

	// Verify file exists
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	// Read file content
	content, err := os.ReadFile(filepath.Join(tmpDir, entries[0].Name()))
	require.NoError(t, err)

	var item orchestrator.WorkItem
	err = json.Unmarshal(content, &item)
	require.NoError(t, err)

	assert.Equal(t, "Fix bug in login", item.Summary)
	assert.Equal(t, "https://github.com/example/repo", item.RepoURL)
	assert.NotEmpty(t, item.ID)
}

func TestTaskAdd_File(t *testing.T) {
	// Setup temp file
	tmpDir := t.TempDir()
	workFile := filepath.Join(tmpDir, "work_items.json")

	// Reset viper
	viper.Reset()
	defer viper.Reset()

	// Test adding a task
	out, err := runTaskCommand(t, "add", "Implement feature X", "--poller", "file", "--work-file", workFile, "--description", "Details here")
	require.NoError(t, err)
	assert.Contains(t, out, "Task added to")

	// Verify file content
	content, err := os.ReadFile(workFile)
	require.NoError(t, err)

	var items []orchestrator.WorkItem
	err = json.Unmarshal(content, &items)
	require.NoError(t, err)
	require.Len(t, items, 1)

	assert.Equal(t, "Implement feature X", items[0].Summary)
	assert.Equal(t, "Details here", items[0].Description)

	// Add another task
	out, err = runTaskCommand(t, "add", "Task 2", "--poller", "file", "--work-file", workFile)
	require.NoError(t, err)

	content, err = os.ReadFile(workFile)
	require.NoError(t, err)
	err = json.Unmarshal(content, &items)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "Task 2", items[1].Summary)
}

func TestTaskList_FileDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a dummy task file
	item := orchestrator.WorkItem{
		ID:      "task-123",
		Summary: "Existing task",
		RepoURL: "http://repo",
	}
	data, _ := json.Marshal(item)
	os.WriteFile(filepath.Join(tmpDir, "task-1.json"), data, 0644)

	// Run list
	out, err := runTaskCommand(t, "list", "--poller", "file-dir", "--watch-dir", tmpDir)
	require.NoError(t, err)
	assert.Contains(t, out, "task-123")
	assert.Contains(t, out, "Existing task")
}

func TestTaskList_ViperConfig(t *testing.T) {
	tmpDir := t.TempDir()
	viper.Reset()
	defer viper.Reset()

	// Set config via viper
	viper.Set("orchestrator.poller", "file-dir")
	viper.Set("orchestrator.watch_dir", tmpDir)

	item := orchestrator.WorkItem{
		ID:      "task-vip",
		Summary: "Viper task",
	}
	data, _ := json.Marshal(item)
	os.WriteFile(filepath.Join(tmpDir, "task-vip.json"), data, 0644)

	// Run list without flags
	out, err := runTaskCommand(t, "list")
	require.NoError(t, err)
	assert.Contains(t, out, "task-vip")
	assert.Contains(t, out, "Viper task")
}
