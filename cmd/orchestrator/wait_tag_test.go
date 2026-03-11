package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWaitForTag_AllCompleted(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		assert.Equal(t, "all", r.URL.Query().Get("state"))
		assert.Equal(t, "deploy", r.URL.Query().Get("tag"))

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
	err := waitForTag(server.URL, "deploy", &buf)

	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Waiting for jobs with tag 'deploy' to complete...")
	assert.Contains(t, buf.String(), "All jobs with tag 'deploy' completed successfully.")
	assert.Equal(t, 2, callCount)
}

func TestWaitForTag_OneFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"id": "job-1", "status": "Completed"},
			{"id": "job-2", "status": "Failed", "error": "exit code 1"}
		]`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	err := waitForTag(server.URL, "deploy", &buf)

	assert.Error(t, err)
	assert.Equal(t, "job job-2 failed with error: exit code 1", err.Error())
	assert.Contains(t, buf.String(), "Waiting for jobs with tag 'deploy' to complete...")
}

func TestWaitForTag_OneCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"id": "job-1", "status": "Completed"},
			{"id": "job-2", "status": "Canceled", "error": "user aborted"}
		]`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	err := waitForTag(server.URL, "deploy", &buf)

	assert.Error(t, err)
	assert.Equal(t, "job job-2 canceled with error: user aborted", err.Error())
}

func TestWaitForTag_NoJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	err := waitForTag(server.URL, "deploy", &buf)

	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "No jobs found with tag 'deploy'.")
}
