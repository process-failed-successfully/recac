package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"recac/internal/agent"
	"recac/internal/telemetry"
)

func TestExplainJobAPI_Success(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	mockSpawner.On("GetLogs", mock.Anything, "TEST-EXPLAIN-1").Return(io.NopCloser(bytes.NewBufferString("log line 1\nlog line 2")), nil)

	orch := New(mockPoller, mockSpawner, 1*time.Second)
	logger := telemetry.NewLogger(false, "test", false)

	jobID := "TEST-EXPLAIN-1"
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:      jobID,
		Status:  "Failed",
		Summary: "Test Failed Job",
		Error:   "simulated error",
		WorkItem: WorkItem{
			ID:      jobID,
			Summary: "Test Failed Job",
		},
	})

	originalNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = originalNewAgentFunc }()

	mockAgent := &agent.MockAgent{}
	mockAgent.SetResponse("This is a simulated explanation.")

	newAgentFunc = func(provider, apiKey, model, instructions, agentName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())

	req := httptest.NewRequest(http.MethodGet, "/jobs/"+jobID+"/explain", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)

	var response map[string]string
	err := json.NewDecoder(res.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "This is a simulated explanation.", response["explanation"])
}

func TestExplainJobAPI_JobNotFound(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	orch := New(mockPoller, mockSpawner, 1*time.Second)
	logger := telemetry.NewLogger(false, "test", false)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())

	req := httptest.NewRequest(http.MethodGet, "/jobs/NON-EXISTENT-JOB/explain", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

func TestExplainBulkJobsAPI_Success(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	mockSpawner.On("GetLogs", mock.Anything, "FAILED-1").Return(io.NopCloser(bytes.NewBufferString("log line 1\nlog line 2")), nil)
	mockSpawner.On("GetLogs", mock.Anything, "FAILED-2").Return(io.NopCloser(bytes.NewBufferString("log line 3\nlog line 4")), nil)

	orch := New(mockPoller, mockSpawner, 1*time.Second)
	logger := telemetry.NewLogger(false, "test", false)

	orch.completedJobs = []JobInfo{
		{
			ID:      "FAILED-1",
			Status:  "Failed",
			Summary: "Test Failed Job 1",
			Error:   "simulated error 1",
			WorkItem: WorkItem{
				ID:      "FAILED-1",
				Summary: "Test Failed Job 1",
				Tags:    []string{"bug"},
			},
		},
		{
			ID:      "FAILED-2",
			Status:  "error",
			Summary: "Test Failed Job 2",
			Error:   "simulated error 2",
			WorkItem: WorkItem{
				ID:      "FAILED-2",
				Summary: "Test Failed Job 2",
				Tags:    []string{"bug", "important"},
			},
		},
		{
			ID:      "SUCCESS-1",
			Status:  "Completed",
			Summary: "Test Success Job",
			WorkItem: WorkItem{
				ID:      "SUCCESS-1",
				Summary: "Test Success Job",
				Tags:    []string{"bug"},
			},
		},
	}

	originalNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = originalNewAgentFunc }()

	mockAgent := &agent.MockAgent{}
	mockAgent.SetResponse("Mock explanation for bulk")

	newAgentFunc = func(provider, apiKey, model, instructions, agentName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())

	// Test by tag
	req := httptest.NewRequest(http.MethodGet, "/jobs/explain/bulk?tag=bug", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	res := w.Result()
	require.Equal(t, http.StatusOK, res.StatusCode)
	var response map[string]map[string]string
	err := json.NewDecoder(res.Body).Decode(&response)
	res.Body.Close()
	require.NoError(t, err)
	exps := response["explanations"]
	require.Len(t, exps, 2)
	assert.Equal(t, "Mock explanation for bulk", exps["FAILED-1"])
	assert.Equal(t, "Mock explanation for bulk", exps["FAILED-2"])
	assert.NotContains(t, exps, "SUCCESS-1")

	// Test by match
	req = httptest.NewRequest(http.MethodGet, "/jobs/explain/bulk?match=simulated%20error%202", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	res = w.Result()
	require.Equal(t, http.StatusOK, res.StatusCode)
	err = json.NewDecoder(res.Body).Decode(&response)
	res.Body.Close()
	require.NoError(t, err)
	exps = response["explanations"]
	require.Len(t, exps, 1)
	assert.Equal(t, "Mock explanation for bulk", exps["FAILED-2"])

	// Test missing params
	req = httptest.NewRequest(http.MethodGet, "/jobs/explain/bulk", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	res = w.Result()
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	res.Body.Close()
}
