package orchestrator

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func setupTestOrchestratorForSearchJobs(t *testing.T) *Orchestrator {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	job1 := JobInfo{
		ID:       "JOB-1",
		Summary:  "Fix issue with login",
		Status:   "Completed",
		WorkItem: WorkItem{ID: "JOB-1", Tags: []string{"backend"}},
	}
	job2 := JobInfo{
		ID:       "JOB-2",
		Summary:  "Add panic recovery",
		Status:   "Failed",
		Error:    "runtime error: panic",
		WorkItem: WorkItem{ID: "JOB-2", Tags: []string{"backend", "urgent"}, Description: "Fix crash"},
	}
	job3 := JobInfo{
		ID:       "JOB-3",
		Summary:  "Update UI",
		Status:   "Running",
		WorkItem: WorkItem{ID: "JOB-3", Tags: []string{"frontend"}},
	}

	orch.mu.Lock()
	orch.completedJobs = append(orch.completedJobs, job1, job2)
	orch.activeJobs = map[string]JobInfo{"JOB-3": job3}
	orch.mu.Unlock()

	return orch
}

func TestAPI_SearchJobs_MatchSummary(t *testing.T) {
	orch := setupTestOrchestratorForSearchJobs(t)
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	req := httptest.NewRequest("GET", "/jobs/search?q=login", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var results []JobInfo
	err := json.NewDecoder(rr.Body).Decode(&results)
	assert.NoError(t, err)

	assert.Len(t, results, 1)
	assert.Equal(t, "JOB-1", results[0].ID)
}

func TestAPI_SearchJobs_MatchErrorAndDescription(t *testing.T) {
	orch := setupTestOrchestratorForSearchJobs(t)
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	req := httptest.NewRequest("GET", "/jobs/search?q=crash|panic", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var results []JobInfo
	err := json.NewDecoder(rr.Body).Decode(&results)
	assert.NoError(t, err)

	assert.Len(t, results, 1)
	assert.Equal(t, "JOB-2", results[0].ID)
}

func TestAPI_SearchJobs_FilterByTag(t *testing.T) {
	orch := setupTestOrchestratorForSearchJobs(t)
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	// Both JOB-1 and JOB-2 are "backend", but only JOB-1 has "login"
	// Actually query "." matches all, filter by tag "backend"
	req := httptest.NewRequest("GET", "/jobs/search?q=.&tag=backend", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var results []JobInfo
	err := json.NewDecoder(rr.Body).Decode(&results)
	assert.NoError(t, err)

	assert.Len(t, results, 2)
}

func TestAPI_SearchJobs_FilterByStatus(t *testing.T) {
	orch := setupTestOrchestratorForSearchJobs(t)
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	req := httptest.NewRequest("GET", "/jobs/search?q=.&status=Running", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var results []JobInfo
	err := json.NewDecoder(rr.Body).Decode(&results)
	assert.NoError(t, err)

	assert.Len(t, results, 1)
	assert.Equal(t, "JOB-3", results[0].ID)
}

func TestAPI_SearchJobs_InvalidRegex(t *testing.T) {
	orch := setupTestOrchestratorForSearchJobs(t)
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	req := httptest.NewRequest("GET", "/jobs/search?q=[invalid", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid regex")
}

func TestAPI_SearchJobs_MissingQuery(t *testing.T) {
	orch := setupTestOrchestratorForSearchJobs(t)
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	req := httptest.NewRequest("GET", "/jobs/search", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "required")
}
