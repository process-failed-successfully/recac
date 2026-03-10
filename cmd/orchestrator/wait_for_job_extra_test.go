package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
)

func TestWaitForJob_StreamLogs(t *testing.T) {
	mux := http.NewServeMux()

	// Track number of calls to simulate state progression
	jobCalls := 0

	mux.HandleFunc("/jobs/JOB-123", func(w http.ResponseWriter, r *http.Request) {
		jobCalls++

		status := "Pending"
		if jobCalls == 2 {
			status = "Running"
		} else if jobCalls >= 3 {
			status = "Completed"
		}

		job := orchestrator.JobInfo{
			ID:     "JOB-123",
			Status: status,
		}
		json.NewEncoder(w).Encode(job)
	})

	mux.HandleFunc("/jobs/JOB-123/logs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("some logs\n"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer

	errCh := make(chan error)
	go func() {
		errCh <- waitForJob(server.URL, "JOB-123", &buf)
	}()

	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for job")
	}

	output := buf.String()
	assert.Contains(t, output, "--- Log Stream Start ---")
	assert.Contains(t, output, "some logs")
	assert.Contains(t, output, "--- Log Stream End ---")
}

func TestWaitForJob_StreamLogs_Failed(t *testing.T) {
	mux := http.NewServeMux()

	jobCalls := 0

	mux.HandleFunc("/jobs/JOB-FAIL", func(w http.ResponseWriter, r *http.Request) {
		jobCalls++

		status := "Running"
		if jobCalls >= 2 {
			status = "Failed"
		}

		job := orchestrator.JobInfo{
			ID:     "JOB-FAIL",
			Status: status,
			Error:  "Something went wrong",
		}
		json.NewEncoder(w).Encode(job)
	})

	mux.HandleFunc("/jobs/JOB-FAIL/logs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("failing logs\n"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer

	errCh := make(chan error)
	go func() {
		errCh <- waitForJob(server.URL, "JOB-FAIL", &buf)
	}()

	select {
	case err := <-errCh:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Something went wrong")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for job")
	}
}

func TestWaitForJob_StreamLogs_Canceled(t *testing.T) {
	mux := http.NewServeMux()

	jobCalls := 0

	mux.HandleFunc("/jobs/JOB-CANCEL", func(w http.ResponseWriter, r *http.Request) {
		jobCalls++

		status := "Running"
		if jobCalls >= 2 {
			status = "Canceled"
		}

		job := orchestrator.JobInfo{
			ID:     "JOB-CANCEL",
			Status: status,
			Error:  "Canceled during run",
		}
		json.NewEncoder(w).Encode(job)
	})

	mux.HandleFunc("/jobs/JOB-CANCEL/logs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("some logs\n"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer

	errCh := make(chan error)
	go func() {
		errCh <- waitForJob(server.URL, "JOB-CANCEL", &buf)
	}()

	select {
	case err := <-errCh:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Canceled during run")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for job")
	}
}

func TestWaitForJob_NetworkErrorRecovery(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			// First time drop connection
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
			return
		}
		if calls == 2 {
			// Second time invalid JSON
			w.Write([]byte("invalid json"))
			return
		}

		// Third time success
		job := orchestrator.JobInfo{
			ID:     "JOB-RECOVER",
			Status: "Completed",
		}
		json.NewEncoder(w).Encode(job)
	}))
	defer server.Close()

	var buf bytes.Buffer

	errCh := make(chan error)
	go func() {
		errCh <- waitForJob(server.URL, "JOB-RECOVER", &buf)
	}()

	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for job")
	}
}
func TestWaitForJob_LogsNotReady(t *testing.T) {
	mux := http.NewServeMux()

	jobCalls := 0

	mux.HandleFunc("/jobs/JOB-LNR", func(w http.ResponseWriter, r *http.Request) {
		jobCalls++

		status := "Running"
		if jobCalls >= 3 {
			status = "Completed"
		}

		job := orchestrator.JobInfo{
			ID:     "JOB-LNR",
			Status: status,
		}
		json.NewEncoder(w).Encode(job)
	})

	logCalls := 0
	mux.HandleFunc("/jobs/JOB-LNR/logs", func(w http.ResponseWriter, r *http.Request) {
		logCalls++
		if logCalls == 1 {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not found"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("some logs\n"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer

	errCh := make(chan error)
	go func() {
		errCh <- waitForJob(server.URL, "JOB-LNR", &buf)
	}()

	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for job")
	}

	output := buf.String()
	assert.Contains(t, output, "--- Log Stream Start ---")
	assert.Contains(t, output, "some logs")
}
func TestLimitString(t *testing.T) {
	assert.Equal(t, "short", limitString("short", 10))
	assert.Equal(t, "exactlyten", limitString("exactlyten", 10))
	assert.Equal(t, "thisisvery...", limitString("thisisverylong", 10))
}
