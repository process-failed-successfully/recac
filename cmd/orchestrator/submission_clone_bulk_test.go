package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCloneBulkJobs_Tag(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/clone/bulk", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "bulk-test-tag", r.URL.Query().Get("tag"))
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"cloned": 2, "cloned_job_ids": ["JOB-1", "JOB-2"]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	stdout = w

	var priority int = 10
	cloneBulkJobs(server.URL, "", "bulk-test-tag", &priority, false, nil, nil)

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Successfully cloned 2 jobs.")
	assert.Contains(t, out, "- JOB-1")
	assert.Contains(t, out, "- JOB-2")
}

func TestCloneBulkJobs_Match(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/clone/bulk", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "regex-test", r.URL.Query().Get("match"))
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"cloned": 1, "cloned_job_ids": ["JOB-3"]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	stdout = w

	cloneBulkJobs(server.URL, "regex-test", "", nil, false, nil, nil)

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Successfully cloned 1 jobs.")
	assert.Contains(t, out, "- JOB-3")
}

func TestCloneBulkJobs_Overrides(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/clone/bulk", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"cloned": 0, "cloned_job_ids": []}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	stdout = w

	priority := 5
	envVars := map[string]string{"TEST": "test"}
	dependsOn := []string{"DEP-1"}
	cloneBulkJobs(server.URL, "test", "", &priority, false, envVars, dependsOn)

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Successfully cloned 0 jobs.")
}

func TestCloneBulkJobs_Error(t *testing.T) {
	// Need to mock exitFunc since submission.go calls it on error
	originalExitFunc := exitFunc
	exitCalled := false
	exitCode := 0
	exitFunc = func(code int) {
		exitCalled = true
		exitCode = code
	}
	defer func() { exitFunc = originalExitFunc }()

	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/clone/bulk", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`invalid query params`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	stdout = w

	cloneBulkJobs(server.URL, "bad-query", "", nil, false, nil, nil)

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to clone jobs: invalid query params")
	assert.True(t, exitCalled)
	assert.Equal(t, 1, exitCode)
}
