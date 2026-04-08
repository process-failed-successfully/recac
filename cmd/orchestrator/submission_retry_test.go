package main

import (
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
