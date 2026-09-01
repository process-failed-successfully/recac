package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWaitForJobs_NetworkErrorRecovery(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/job-1" {
			callCount++
			if callCount == 1 {
				// 1. Connection error / Bad JSON
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{invalid-json`))
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "Completed"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	var buf bytes.Buffer
	err := waitForJobs(server.URL, []string{"job-1"}, &buf)

	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Waiting for 1 jobs to complete...")
	assert.Contains(t, buf.String(), "All 1 jobs completed successfully.")
	assert.GreaterOrEqual(t, callCount, 2)
}

func TestWaitForJobs_Canceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "Canceled", "error": "user aborted"}`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	err := waitForJobs(server.URL, []string{"job-cancel"}, &buf)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "canceled with error: user aborted")
}
