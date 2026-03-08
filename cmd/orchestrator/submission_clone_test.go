package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloneJob_Success(t *testing.T) {
	var capturedOverrides map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/orig-123/clone", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		if r.Body != nil {
			err := json.NewDecoder(r.Body).Decode(&capturedOverrides)
			require.NoError(t, err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"cloned_job_id": "new-123"}`))
	}))
	defer server.Close()

	var out bytes.Buffer
	stdout = &out

	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}

	priority := 10
	envVars := map[string]string{"K1": "V1"}
	dependsOn := []string{"dep-1"}

	cloneJob(server.URL, "orig-123", "new-123", &priority, false, envVars, dependsOn)

	assert.False(t, exitCalled)
	assert.Contains(t, out.String(), "cloned successfully as new-123")

	require.NotNil(t, capturedOverrides)
	assert.Equal(t, "new-123", capturedOverrides["new_id"])

	// Convert priority back since JSON parses it as float64
	p, ok := capturedOverrides["priority"].(float64)
	assert.True(t, ok)
	assert.Equal(t, 10.0, p)

	env, ok := capturedOverrides["env_vars"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "V1", env["K1"])

	deps, ok := capturedOverrides["depends_on"].([]interface{})
	assert.True(t, ok)
	assert.Equal(t, "dep-1", deps[0])
}

func TestCloneJob_NoOverrides(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/orig-123/clone", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"cloned_job_id": "orig-123-clone-12345"}`))
	}))
	defer server.Close()

	var out bytes.Buffer
	stdout = &out

	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}

	cloneJob(server.URL, "orig-123", "", nil, false, nil, nil)

	assert.False(t, exitCalled)
	assert.Contains(t, out.String(), "cloned successfully as orig-123-clone-12345")
}

func TestCloneJob_FailedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`Internal Server Error`))
	}))
	defer server.Close()

	var out bytes.Buffer
	stdout = &out

	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}

	cloneJob(server.URL, "orig-123", "", nil, false, nil, nil)

	assert.True(t, exitCalled)
	assert.Contains(t, out.String(), "Failed to clone job")
}
