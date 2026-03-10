package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListJobs_MatchFilter(t *testing.T) {
	// Start a mock server to intercept the API request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)

		// Assert that the 'match' query parameter was passed correctly
		matchParam := r.URL.Query().Get("match")
		assert.Equal(t, "timeout", matchParam)

		// Assert other parameters
		assert.Equal(t, "all", r.URL.Query().Get("state"))
		assert.Equal(t, "Failed", r.URL.Query().Get("status"))
		assert.Equal(t, "urgent", r.URL.Query().Get("tag"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id": "job-1", "summary": "Failed due to timeout", "status": "Failed"}]`))
	}))
	defer server.Close()

	// Intercept stdout to prevent spamming test output
	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	// Execute listJobs with the match filter
	listJobs(server.URL, true, "Failed", "urgent", "timeout")

	// Read and verify stdout
	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "job-1")
	assert.Contains(t, output, "Failed due to timeout")
}
