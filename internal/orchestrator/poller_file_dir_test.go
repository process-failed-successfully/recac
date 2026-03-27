package orchestrator

import (
	"io"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileDirPoller_New(t *testing.T) {
	tempDir := t.TempDir()
	poller, err := NewFileDirPoller(tempDir)
	require.NoError(t, err)
	assert.NotNil(t, poller)

	// Check if processed directory was created
	processedDir := filepath.Join(tempDir, "processed")
	info, err := os.Stat(processedDir)
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestFileDirPoller_Poll(t *testing.T) {
	tempDir := t.TempDir()
	processedDir := filepath.Join(tempDir, "processed")
	require.NoError(t, os.MkdirAll(processedDir, 0755))

	poller, err := NewFileDirPoller(tempDir)
	require.NoError(t, err)

	// Create a valid work item file
	item := WorkItem{
		ID:          "task-1",
		Summary:     "Test Task",
		Description: "Do something",
		EnvVars:     map[string]string{"foo": "bar"},
	}
	itemData, err := json.Marshal(item)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "task1.json"), itemData, 0644)
	require.NoError(t, err)

	// Create an invalid JSON file
	err = os.WriteFile(filepath.Join(tempDir, "invalid.json"), []byte("{invalid"), 0644)
	require.NoError(t, err)

	// Create a non-JSON file
	err = os.WriteFile(filepath.Join(tempDir, "other.txt"), []byte("text"), 0644)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	// Poll
	items, err := poller.Poll(ctx, logger)
	require.NoError(t, err)

	// Verify we got 1 valid item
	assert.Len(t, items, 1)
	assert.Equal(t, "task-1", items[0].ID)
	assert.Equal(t, "Test Task", items[0].Summary)

	// Verify file movement
	// task1.json should be in processed
	_, err = os.Stat(filepath.Join(processedDir, "task1.json"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(tempDir, "task1.json"))
	assert.True(t, os.IsNotExist(err))

	// invalid.json should remain
	_, err = os.Stat(filepath.Join(tempDir, "invalid.json"))
	assert.NoError(t, err)

	// other.txt should remain
	_, err = os.Stat(filepath.Join(tempDir, "other.txt"))
	assert.NoError(t, err)
}

func TestFileDirPoller_UpdateStatus(t *testing.T) {
	tempDir := t.TempDir()
	poller, _ := NewFileDirPoller(tempDir)

	item := WorkItem{ID: "task-1"}
	err := poller.UpdateStatus(context.Background(), item, "completed", "done")
	assert.NoError(t, err)
}

func TestFileDirPoller_Poll_ReadDirError(t *testing.T) {
	// Use a non-existent directory to force error
	poller := &FileDirPoller{
		watchDir: "/path/to/non/existent/dir",
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	_, err := poller.Poll(context.Background(), logger)
	assert.Error(t, err)
}

func TestFileDirPoller_Ping_FileInsteadOfDir(t *testing.T) {
	tempDir := t.TempDir()
	f, err := os.CreateTemp(tempDir, "test")
	require.NoError(t, err)
	f.Close()
	poller := &FileDirPoller{watchDir: f.Name()}
	err = poller.Ping(context.Background())
	assert.Error(t, err)
}

func TestFileDirPoller_Poll_MoveFail(t *testing.T) {
	tempDir := t.TempDir()
	poller, err := NewFileDirPoller(tempDir)
	require.NoError(t, err)

	// make processed dir unwritable
	processedDir := filepath.Join(tempDir, "processed")
	err = os.Chmod(processedDir, 0555)
	require.NoError(t, err)
	defer os.Chmod(processedDir, 0755)

	item := WorkItem{ID: "task-1"}
	itemData, _ := json.Marshal(item)
	err = os.WriteFile(filepath.Join(tempDir, "task1.json"), itemData, 0644)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	items, err := poller.Poll(context.Background(), logger)
	assert.NoError(t, err)
	assert.Empty(t, items)
}

func TestFileDirPoller_Poll_ReadFail(t *testing.T) {
	tempDir := t.TempDir()
	poller, err := NewFileDirPoller(tempDir)
	require.NoError(t, err)

	itemFile := filepath.Join(tempDir, "task1.json")
	err = os.WriteFile(itemFile, []byte("{}"), 0000)
	require.NoError(t, err)
	defer os.Chmod(itemFile, 0644)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	items, err := poller.Poll(context.Background(), logger)
	assert.NoError(t, err)
	assert.Empty(t, items)
}

func TestFileDirPoller_New_Error(t *testing.T) {
	tempDir := t.TempDir()
	invalidDir := filepath.Join(tempDir, "new")
	err := os.MkdirAll(invalidDir, 0755) // Read and execute only, no write
	require.NoError(t, err)

	// Create a file in that directory so processed can't be created
	conflictPath := filepath.Join(invalidDir, "processed")
	err = os.WriteFile(conflictPath, []byte(""), 0644)
	require.NoError(t, err)

	poller, err := NewFileDirPoller(invalidDir)
	assert.Error(t, err)
	assert.Nil(t, poller)
}


func TestFileDirPoller_Poll_EmptyDir(t *testing.T) {
	tempDir := t.TempDir()
	poller, err := NewFileDirPoller(tempDir)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	items, err := poller.Poll(context.Background(), logger)
	assert.NoError(t, err)
	assert.Empty(t, items)
}

func TestFileDirPoller_Poll_IgnoresDirAndNonJson(t *testing.T) {
	tempDir := t.TempDir()
	poller, err := NewFileDirPoller(tempDir)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(tempDir, "subdir.json"), 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("test"), 0644)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	items, err := poller.Poll(context.Background(), logger)
	assert.NoError(t, err)
	assert.Empty(t, items)
}

func TestFileDirPoller_PipelineYAML(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()
	dir := t.TempDir()

	poller, err := NewFileDirPoller(dir)
	require.NoError(t, err)

	templateContent := `
name: Dir Pipeline
jobs:
  job1:
    summary: Job 1
  job2:
    summary: Job 2
    depends_on: [job1]
`
	err = os.WriteFile(filepath.Join(dir, "pipeline.yaml"), []byte(templateContent), 0644)
	require.NoError(t, err)

	items, err := poller.Poll(ctx, logger)
	assert.NoError(t, err)
	assert.Len(t, items, 2, "Should return 2 jobs from pipeline")

	// Verify file was moved to processed
	_, err = os.Stat(filepath.Join(dir, "pipeline.yaml"))
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(filepath.Join(dir, "processed", "pipeline.yaml"))
	assert.NoError(t, err)

	jobMap := make(map[string]WorkItem)
	for _, item := range items {
		jobMap[item.Summary] = item
	}

	assert.Contains(t, jobMap, "Job 1")
	assert.Contains(t, jobMap, "Job 2")

	job2 := jobMap["Job 2"]
	assert.Len(t, job2.DependsOn, 1)
	assert.Equal(t, jobMap["Job 1"].ID, job2.DependsOn[0])

	// Poll again should be empty
	items2, err := poller.Poll(ctx, logger)
	assert.NoError(t, err)
	assert.Empty(t, items2)
}
