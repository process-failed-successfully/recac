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
