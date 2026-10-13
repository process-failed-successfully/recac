package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWaitForTag_NetworkErrorRecovery(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// Simulate a temporary server error
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if callCount == 2 {
			// Simulate invalid JSON format
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{invalid json}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"id": "job-1", "status": "Completed"}
		]`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	err := waitForTag(server.URL, "deploy", &buf)

	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Waiting for jobs with tag 'deploy' to complete...")
	assert.Contains(t, buf.String(), "All jobs with tag 'deploy' completed successfully.")
	assert.GreaterOrEqual(t, callCount, 3)
}
