package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApproveBulkJobs_Errors(t *testing.T) {
	origExit := exitFunc
	defer func() { exitFunc = origExit }()
	var exitCode int
	exitFunc = func(code int) { exitCode = code }

	origStdout := stdout
	defer func() { stdout = origStdout }()

	t.Run("InvalidURL", func(t *testing.T) {
		var buf bytes.Buffer
		stdout = &buf
		exitCode = 0

		approveBulkJobs("http://\x00invalid", "", "", "", "")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to parse URL")
	})

	t.Run("BadJSONResponse", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{bad json}`))
		}))
		defer server.Close()

		var buf bytes.Buffer
		stdout = &buf
		exitCode = 0

		approveBulkJobs(server.URL, "", "", "", "")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to decode response")
	})
}
