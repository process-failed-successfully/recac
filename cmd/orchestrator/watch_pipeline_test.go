package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"recac/internal/orchestrator"
)

func TestWatchPipelineJob(t *testing.T) {
	// Setup mock orchestrator server
	var postCount int32
	var getPendingCount int32
	var getActiveCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs" && r.Method == http.MethodPost {
			atomic.AddInt32(&postCount, 1)
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"status":"accepted"}`))
			return
		}
		if r.URL.Path == "/jobs" && r.Method == http.MethodGet {
			state := r.URL.Query().Get("state")
			if state == "active" {
				atomic.AddInt32(&getActiveCount, 1)
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode([]orchestrator.JobInfo{})
				return
			} else if state == "pending" {
				atomic.AddInt32(&getPendingCount, 1)
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode([]orchestrator.JobInfo{})
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Create temp pipeline file
	tmpFile, err := os.CreateTemp("", "pipeline_watch_*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	yamlContent1 := `
name: WatchPipeline1
jobs:
  job1:
    summary: "Job 1"
    task: "echo 1"
`
	_, err = tmpFile.Write([]byte(yamlContent1))
	require.NoError(t, err)
	tmpFile.Close()

	// Start watcher in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		watchPipelineJob(ctx, server.URL, tmpFile.Name(), false, "", nil, "stable", 50*time.Millisecond)
		close(done)
	}()

	// Wait for initial apply
	time.Sleep(100 * time.Millisecond)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&postCount), int32(1), "Initial apply should post a job")

	// Modify the file to trigger watcher
	yamlContent2 := `
name: WatchPipeline2
jobs:
  job1:
    summary: "Job 1 Modified"
    task: "echo 1"
`
	// Ensure modtime is noticeably different
	time.Sleep(100 * time.Millisecond)
	err = os.WriteFile(tmpFile.Name(), []byte(yamlContent2), 0644)
	require.NoError(t, err)

	// Wait for watcher to trigger
	time.Sleep(200 * time.Millisecond)

	// Cancel the context to stop watcher
	cancel()
	<-done // Wait for goroutine to exit

	// Expect postCount to have increased due to the re-apply logic sending an update or create
	// (Actually apply_pipeline with "stable" runID will try to PUT if the job exists, or POST if it doesn't.
	// Since we return empty arrays for /jobs GET, it will think it doesn't exist and POST again).
	assert.GreaterOrEqual(t, atomic.LoadInt32(&postCount), int32(2), "Watcher should have re-applied and posted again")

	output := buf.String()
	assert.Contains(t, output, "Watching Pipeline:")
	assert.Contains(t, output, "Detected change in")
}

func TestWatchPipelineJob_ErrorResilience(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	tmpFile, err := os.CreateTemp("", "pipeline_watch_err_*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Start with invalid YAML
	err = os.WriteFile(tmpFile.Name(), []byte("invalid: yaml: content: ["), 0644)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		watchPipelineJob(ctx, "http://localhost", tmpFile.Name(), false, "", nil, "stable", 50*time.Millisecond)
		close(done)
	}()

	// Wait for initial apply (which should fail but not panic/exit)
	time.Sleep(100 * time.Millisecond)

	// Change to valid YAML
	validYaml := `
name: WatchPipelineValid
jobs:
  job1:
    summary: "Job 1"
    task: "echo 1"
`
	err = os.WriteFile(tmpFile.Name(), []byte(validYaml), 0644)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
	cancel()
	<-done

	output := buf.String()
	assert.Contains(t, output, "Error applying pipeline: Pipeline validation failed")
	assert.Contains(t, output, "Detected change in")
}
