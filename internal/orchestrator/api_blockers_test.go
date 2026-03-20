package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"recac/internal/telemetry"
)

func TestGetJobBlockersAPI_Success(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	orch := New(mockPoller, mockSpawner, 1*time.Second)
	logger := telemetry.NewLogger(false, "test", false)

	// Setup data
	jobID := "TEST-BLOCKER-1"
	depID := "PENDING-DEP-1"

	orch.pendingJobs[depID] = JobInfo{
		ID:     depID,
		Status: "Pending",
	}

	orch.activeJobs[jobID] = JobInfo{
		ID: jobID,
		WorkItem: WorkItem{
			ID:        jobID,
			DependsOn: []string{depID},
		},
	}

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())

	req := httptest.NewRequest(http.MethodGet, "/jobs/"+jobID+"/blockers", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)

	var response []JobInfo
	err := json.NewDecoder(res.Body).Decode(&response)
	require.NoError(t, err)

	assert.Len(t, response, 1)
	assert.Equal(t, depID, response[0].ID)
	assert.Equal(t, "Pending", response[0].Status)
}

func TestGetJobBlockersAPI_Empty(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	orch := New(mockPoller, mockSpawner, 1*time.Second)
	logger := telemetry.NewLogger(false, "test", false)

	jobID := "TEST-BLOCKER-2"

	orch.activeJobs[jobID] = JobInfo{
		ID: jobID,
		WorkItem: WorkItem{
			ID: jobID,
		},
	}

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())

	req := httptest.NewRequest(http.MethodGet, "/jobs/"+jobID+"/blockers", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)

	var response []JobInfo
	err := json.NewDecoder(res.Body).Decode(&response)
	require.NoError(t, err)

	assert.Empty(t, response)
}

func TestGetJobBlockersAPI_NotFound(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	orch := New(mockPoller, mockSpawner, 1*time.Second)
	logger := telemetry.NewLogger(false, "test", false)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())

	req := httptest.NewRequest(http.MethodGet, "/jobs/NON_EXISTENT_JOB/blockers", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	require.Equal(t, http.StatusNotFound, res.StatusCode)
}
