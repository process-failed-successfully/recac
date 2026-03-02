package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmitAdHocJob(t *testing.T) {
	// 1. Setup mock server
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var err error
		receivedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Job submitted"))
	}))
	defer server.Close()

	// 2. Call function
	envVars := map[string]string{
		"KEY1": "VALUE1",
		"KEY2": "VALUE2",
	}
	submitAdHocJob(server.URL, "http://repo.com", "My Task", "MY-ID", false, envVars)

	// 3. Verify payload
	var item orchestrator.WorkItem
	err := json.Unmarshal(receivedBody, &item)
	require.NoError(t, err)

	assert.Equal(t, "MY-ID", item.ID)
	assert.Equal(t, "http://repo.com", item.RepoURL)
	assert.Equal(t, "My Task", item.Summary)
	assert.Equal(t, "My Task", item.Description)
	assert.Equal(t, envVars, item.EnvVars)
}

func TestSubmitAdHocJob_AutoID(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	submitAdHocJob(server.URL, "http://repo.com", "My Task", "", false, nil)

	var item orchestrator.WorkItem
	err := json.Unmarshal(receivedBody, &item)
	require.NoError(t, err)

	assert.NotEmpty(t, item.ID)
	assert.Equal(t, "http://repo.com", item.RepoURL)
}
