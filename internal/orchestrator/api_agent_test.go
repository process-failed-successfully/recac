package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUpdateJobAgentAPI(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(mockPoller, mockSpawner, 1*time.Minute)

	// Add a pending job
	jobID := "JOB-API-AGENT-1"
	orch.pendingJobs[jobID] = JobInfo{
		ID:     jobID,
		Status: "Pending",
		WorkItem: WorkItem{
			ID:       jobID,
			RunAfter: time.Now().Add(1 * time.Hour), // Prevent evaluatePendingJobs from eating it
		},
	}

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())

	server := httptest.NewServer(mux)
	defer server.Close()

	// 1. Valid Request
	reqBody := `{"agent_provider": "openai", "agent_model": "gpt-4o"}`
	req, err := http.NewRequest(http.MethodPut, server.URL+"/jobs/"+jobID+"/agent", bytes.NewReader([]byte(reqBody)))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var respData map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&respData)
	assert.NoError(t, err)
	assert.Equal(t, "openai", respData["agent_provider"])
	assert.Equal(t, "gpt-4o", respData["agent_model"])

	// Verify internal state
	orch.mu.RLock()
	updatedJob := orch.pendingJobs[jobID]
	orch.mu.RUnlock()
	assert.Equal(t, "openai", updatedJob.WorkItem.AgentProvider)
	assert.Equal(t, "gpt-4o", updatedJob.WorkItem.AgentModel)

	// 2. Invalid Job ID
	req, err = http.NewRequest(http.MethodPut, server.URL+"/jobs/NON-EXISTENT/agent", bytes.NewReader([]byte(reqBody)))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// 3. Active Job
	activeJobID := "JOB-ACTIVE-1"
	orch.mu.Lock()
	orch.activeJobs[activeJobID] = JobInfo{
		ID:     activeJobID,
		Status: "Running",
	}
	orch.mu.Unlock()

	req, err = http.NewRequest(http.MethodPut, server.URL+"/jobs/"+activeJobID+"/agent", bytes.NewReader([]byte(reqBody)))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	// 4. Invalid JSON
	req, err = http.NewRequest(http.MethodPut, server.URL+"/jobs/"+jobID+"/agent", bytes.NewReader([]byte(`{"invalid json`)))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
