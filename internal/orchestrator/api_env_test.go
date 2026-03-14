package orchestrator

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPI_UpdateJobEnv(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := new(MockSpawner)
	orch := New(poller, spawner, 1*time.Minute)
	orch.RequireApproval = true

	orch.mu.Lock()
	orch.pendingJobs["JOB-1"] = JobInfo{
		ID:       "JOB-1",
		Status:   "Pending Approval",
		WorkItem: WorkItem{ID: "JOB-1"},
	}
	orch.mu.Unlock()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, nil)
	server := httptest.NewServer(mux)
	defer server.Close()

	reqBody := struct {
		EnvVars map[string]string `json:"env_vars"`
	}{
		EnvVars: map[string]string{"NEW_ENV": "VALUE123"},
	}

	payload, _ := json.Marshal(reqBody)

	req, err := http.NewRequest(http.MethodPut, server.URL+"/jobs/JOB-1/env", bytes.NewReader(payload))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	orch.mu.Lock()
	job := orch.pendingJobs["JOB-1"]
	orch.mu.Unlock()

	assert.Equal(t, "VALUE123", job.WorkItem.EnvVars["NEW_ENV"])
}
