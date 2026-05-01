package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryJob_WithOverrides(t *testing.T) {
	// Setup mock server
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/JOB-123/retry", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var err error
		receivedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Job retry submitted"))
	}))
	defer server.Close()

	// Redirect stdout to capture output
	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() {
		stdout = oldStdout
	}()

	envVars := map[string]string{
		"NEW_VAR": "NEW_VAL",
	}

	retryJob(server.URL, "JOB-123", false, envVars, "new-provider", "new-model")

	pw.Close()
	io.ReadAll(pr) // ignore output for this test

	// Verify payload
	var reqBody map[string]interface{}
	err := json.Unmarshal(receivedBody, &reqBody)
	require.NoError(t, err)

	assert.Equal(t, "new-provider", reqBody["agent_provider"])
	assert.Equal(t, "new-model", reqBody["agent_model"])

	envVarsPayload := reqBody["env_vars"].(map[string]interface{})
	assert.Equal(t, "NEW_VAL", envVarsPayload["NEW_VAR"])
}

func TestRetryJob_Errors(t *testing.T) {
	origExit := exitFunc
	defer func() { exitFunc = origExit }()
	var exitCode int
	exitFunc = func(code int) { exitCode = code }

	origStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = origStdout }()

	t.Run("InvalidURL", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		retryJob("http://\x00invalid", "JOB-123", false, nil, "", "")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to create request")
	})

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		retryJob("http://invalid-host:12345", "JOB-123", false, nil, "", "")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("ServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`Internal Server Error`))
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		retryJob(server.URL, "JOB-123", false, nil, "", "")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to retry job")
	})
}
