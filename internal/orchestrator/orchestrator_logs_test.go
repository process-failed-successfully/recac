package orchestrator

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestOrchestratorLogs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "orchestrator_logs_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	mockPoller := newMockPoller([]WorkItem{})

	// Create mock spawner
	mockSpawner := new(MockSpawner)

	orch := New(mockPoller, mockSpawner, 1*time.Minute)
	orch.LogDir = tempDir

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Test 1: spawnWorker correctly creates and compresses log
	item := WorkItem{ID: "JOB-LOG", Summary: "Test logs"}

	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)

	logReader := io.NopCloser(bytes.NewBufferString("This is a test log\nLine 2\nLine 3"))
	mockSpawner.On("GetLogs", mock.Anything, "JOB-LOG").Return(logReader, nil)

	orch.wg.Add(1)
	orch.spawnWorker(context.Background(), item, logger)

	// Check log exists
	expectedLogPath := filepath.Join(tempDir, "JOB-LOG.log.gz")
	assert.FileExists(t, expectedLogPath)

	// Test 2: GetLogs returns decompressed logs from persistence
	reader, err := orch.GetLogs(context.Background(), "JOB-LOG")
	require.NoError(t, err)
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "This is a test log\nLine 2\nLine 3", string(decompressed))

	// Test 3: Fallback mechanism when persistent file does not exist
	mockSpawner.On("GetLogs", mock.Anything, "JOB-MISSING").Return(io.NopCloser(bytes.NewBufferString("Missing logs via fallback")), nil)

	readerFallback, err := orch.GetLogs(context.Background(), "JOB-MISSING")
	assert.NoError(t, err)
	defer readerFallback.Close()

	fallbackDecompressed, err := io.ReadAll(readerFallback)
	require.NoError(t, err)
	assert.Equal(t, "Missing logs via fallback", string(fallbackDecompressed))

	// Ensure that path traversal attempts fallback to safe fallback, and return correctly
	// If path is "../JOB-TRAVERSAL", safeID is "JOB-TRAVERSAL", so it looks for JOB-TRAVERSAL.log.gz.
	// Since that doesn't exist, it falls back to spawner GetLogs with original jobID "../JOB-TRAVERSAL".
	mockSpawner.On("GetLogs", mock.Anything, "../JOB-TRAVERSAL").Return(io.NopCloser(bytes.NewBufferString("traversal handled")), nil)
	readerTraversal, err := orch.GetLogs(context.Background(), "../JOB-TRAVERSAL")
	require.NoError(t, err)
	defer readerTraversal.Close()

	traversalDecompressed, err := io.ReadAll(readerTraversal)
	require.NoError(t, err)
	assert.Equal(t, "traversal handled", string(traversalDecompressed))
}
