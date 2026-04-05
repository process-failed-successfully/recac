package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
)

func TestTailSingleJob_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-TAIL-1/logs", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			fmt.Fprintln(w, "Single Log 1")
			return
		}

		fmt.Fprintf(w, "Single Log 1\n")
		flusher.Flush()
		time.Sleep(100 * time.Millisecond)

		fmt.Fprintf(w, "Single Log 2\n")
		flusher.Flush()
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() {
		stdout = oldStdout
	}()

	err := tailSingleJob(context.Background(), server.URL, "JOB-TAIL-1")
	assert.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Starting Log Stream (tailing job JOB-TAIL-1")
	assert.Contains(t, output, "Started Tailing Logs")
	assert.Contains(t, output, "Single Log 1")
	assert.Contains(t, output, "Single Log 2")
	assert.Contains(t, output, "Log Stream Finished")
}

func TestTailSingleJob_ContextCancelled(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-TAIL-2/logs", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if ok {
			fmt.Fprintf(w, "Single Log 1\n")
			flusher.Flush()
		}
		// Block to simulate long running log stream
		time.Sleep(2 * time.Second)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() {
		stdout = oldStdout
	}()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := tailSingleJob(ctx, server.URL, "JOB-TAIL-2")
	assert.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Shutting down stream")
}

func TestTailActiveJobs(t *testing.T) {
	// Create a mock server that simulates an orchestrator
	mux := http.NewServeMux()

	var mu sync.Mutex
	callCount := 0

	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		callCount++

		var jobs []orchestrator.JobInfo

		if callCount == 1 {
			// First poll: return one active job
			jobs = append(jobs, orchestrator.JobInfo{
				ID:     "JOB-1",
				Status: "Active",
			})
		} else if callCount == 2 {
			// Second poll: return two active jobs
			jobs = append(jobs, orchestrator.JobInfo{
				ID:     "JOB-1",
				Status: "Active",
			})
			jobs = append(jobs, orchestrator.JobInfo{
				ID:     "JOB-2",
				Status: "Spawning",
			})
		} else {
			// Third poll: all done
			jobs = append(jobs, orchestrator.JobInfo{
				ID:     "JOB-1",
				Status: "Completed",
			})
			jobs = append(jobs, orchestrator.JobInfo{
				ID:     "JOB-2",
				Status: "Failed",
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jobs)
	})

	mux.HandleFunc("/jobs/JOB-1/logs", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			fmt.Fprintln(w, "Log 1 for JOB-1")
			return
		}

		fmt.Fprintf(w, "Log 1 for JOB-1\n")
		flusher.Flush()
		time.Sleep(100 * time.Millisecond)

		fmt.Fprintf(w, "Log 2 for JOB-1\n")
		flusher.Flush()
	})

	mux.HandleFunc("/jobs/JOB-2/logs", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			fmt.Fprintln(w, "Log 1 for JOB-2")
			return
		}

		fmt.Fprintf(w, "Log 1 for JOB-2\n")
		flusher.Flush()
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Redirect stdout to capture output
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() {
		stdout = oldStdout
	}()

	// We use a short timeout context to kill the tailer since it loops indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()

	// Run the multiplexer
	err := tailActiveJobs(ctx, server.URL, "", "")
	assert.NoError(t, err)

	output := out.String()
	// Assert connection and both jobs
	assert.Contains(t, output, "Starting Log Multiplexer")
	assert.Contains(t, output, "Started Tailing Logs")
	assert.Contains(t, output, "JOB-1")
	assert.Contains(t, output, "Log 1 for JOB-1")
	assert.Contains(t, output, "Log 2 for JOB-1")

	// JOB-2 should be discovered on the second poll
	assert.Contains(t, output, "JOB-2")
	assert.Contains(t, output, "Log 1 for JOB-2")
}

func TestTailActiveJobs_WithFilters(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "active", r.URL.Query().Get("state"))
		assert.Equal(t, "my-tag", r.URL.Query().Get("tag"))
		assert.Equal(t, "my-match", r.URL.Query().Get("match"))

		var jobs []orchestrator.JobInfo
		// Return no jobs so it just exits cleanly
		json.NewEncoder(w).Encode(jobs)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() {
		stdout = oldStdout
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := tailActiveJobs(ctx, server.URL, "my-tag", "my-match")
	assert.NoError(t, err)
}

func TestTailJob_NetworkRetry(t *testing.T) {
	mux := http.NewServeMux()
	callCount := 0

	mux.HandleFunc("/jobs/JOB-RETRY/logs", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusNotFound) // Simulate container not ready
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Recovered stream\n"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() {
		stdout = oldStdout
	}()

	m := &multiplexer{
		host:   server.URL,
		active: make(map[string]context.CancelFunc),
	}
	m.active["JOB-RETRY"] = func() {}

	m.wg.Add(1)
	m.tailJob(context.Background(), "JOB-RETRY", "[JOB-RETRY]")
	m.wg.Wait()

	output := out.String()
	assert.Contains(t, output, "Started Tailing Logs")
	assert.Contains(t, output, "Recovered stream")
	assert.Contains(t, output, "Log Stream Finished")
}

func TestMultiplexer_Poll_NetworkError(t *testing.T) {
	// A server that returns bad JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() {
		stdout = oldStdout
	}()

	m := &multiplexer{
		host:   server.URL,
		active: make(map[string]context.CancelFunc),
	}

	m.poll(context.Background())
	assert.Contains(t, out.String(), "Error decoding jobs")
}

func TestPoll_InvalidHostURL(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	m := &multiplexer{
		host:   "http://\x00invalid",
		active: make(map[string]context.CancelFunc),
	}

	m.poll(context.Background())
	assert.Contains(t, out.String(), "Failed to parse host URL")
}

func TestPoll_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	m := &multiplexer{
		host:   "http://localhost:12345",
		active: make(map[string]context.CancelFunc),
	}

	m.poll(context.Background())
	assert.Contains(t, out.String(), "[Multiplexer] Error fetching jobs")
}

func TestPoll_BadStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	m := &multiplexer{
		host:   server.URL,
		active: make(map[string]context.CancelFunc),
	}

	m.poll(context.Background())
	assert.Contains(t, out.String(), "[Multiplexer] Error fetching jobs: status 500")
}

func TestTailJob_MaxRetriesExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	m := &multiplexer{
		host:   server.URL,
		active: make(map[string]context.CancelFunc),
	}
	m.active["JOB-FAIL"] = func() {}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Shorten the retry sleep locally to make the test fast
	// The maxRetries loop is inside the function so it will run 10 times then abort
	m.wg.Add(1)
	go m.tailJob(ctx, "JOB-FAIL", "[JOB-FAIL]")

	// we wait for it to finish or fail
	m.wg.Wait()
	assert.Contains(t, out.String(), "Failed to fetch logs (status 500) after 10 retries")
}

func TestTailJob_ConnectionErrorRetries(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	m := &multiplexer{
		host:   "http://localhost:12345", // Assuming this will refuse connection
		active: make(map[string]context.CancelFunc),
	}
	m.active["JOB-CONN-FAIL"] = func() {}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.wg.Add(1)
	go m.tailJob(ctx, "JOB-CONN-FAIL", "[JOB-CONN-FAIL]")

	m.wg.Wait()
	assert.Contains(t, out.String(), "Failed to connect to log stream after 10 retries")
}

func TestTailJob_ContextCancelledDuringRequest(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	m := &multiplexer{
		host:   "http://localhost:12345",
		active: make(map[string]context.CancelFunc),
	}
	m.active["JOB-CANCEL"] = func() {}

	ctx, cancel := context.WithCancel(context.Background())
	// cancel immediately
	cancel()

	m.wg.Add(1)
	m.tailJob(ctx, "JOB-CANCEL", "[JOB-CANCEL]")

	// The context check should make it return immediately
	assert.NotContains(t, out.String(), "Failed to connect to log stream after 10 retries")
}
