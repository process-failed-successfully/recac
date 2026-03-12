package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWaitForMatch_AllCompleted(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		assert.Equal(t, "all", r.URL.Query().Get("state"))
		assert.Equal(t, "deploy.*", r.URL.Query().Get("match"))

		callCount++
		w.WriteHeader(http.StatusOK)

		if callCount == 1 {
			// First call: jobs are pending/running
			w.Write([]byte(`[
				{"id": "job-1", "status": "Running"},
				{"id": "job-2", "status": "Pending"}
			]`))
		} else {
			// Second call: all completed
			w.Write([]byte(`[
				{"id": "job-1", "status": "Completed"},
				{"id": "job-2", "status": "Completed"}
			]`))
		}
	}))
	defer server.Close()

	var buf bytes.Buffer
	err := waitForMatch(server.URL, "deploy.*", &buf)

	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Waiting for jobs matching 'deploy.*' to complete...")
	assert.Contains(t, buf.String(), "All jobs matching 'deploy.*' completed successfully.")
	assert.Equal(t, 2, callCount)
}

func TestWaitForMatch_OneFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"id": "job-1", "status": "Completed"},
			{"id": "job-2", "status": "Failed", "error": "exit code 1"}
		]`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	err := waitForMatch(server.URL, "deploy.*", &buf)

	assert.Error(t, err)
	assert.Equal(t, "job job-2 failed with error: exit code 1", err.Error())
	assert.Contains(t, buf.String(), "Waiting for jobs matching 'deploy.*' to complete...")
}

func TestWaitForMatch_OneCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"id": "job-1", "status": "Completed"},
			{"id": "job-2", "status": "Canceled", "error": "user aborted"}
		]`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	err := waitForMatch(server.URL, "deploy.*", &buf)

	assert.Error(t, err)
	assert.Equal(t, "job job-2 canceled with error: user aborted", err.Error())
}

func TestWaitForMatch_NoJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	err := waitForMatch(server.URL, "deploy.*", &buf)

	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "No jobs found matching 'deploy.*'.")
}
