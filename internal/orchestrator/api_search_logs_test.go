package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupTestOrchestratorForLogs(t *testing.T) (*Orchestrator, *MockSpawner) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	job1 := JobInfo{
		ID:        "JOB-1",
		Summary:   "Job 1",
		Status:    "Completed",
		WorkItem:  WorkItem{ID: "JOB-1", Tags: []string{"tag1"}},
	}
	job2 := JobInfo{
		ID:        "JOB-2",
		Summary:   "Job 2",
		Status:    "Failed",
		WorkItem:  WorkItem{ID: "JOB-2", Tags: []string{"tag2"}},
	}

	orch.mu.Lock()
	orch.completedJobs = []JobInfo{job1, job2}
	orch.mu.Unlock()

	return orch, mockSpawner
}

func TestAPI_SearchLogs_MatchFound(t *testing.T) {
	orch, mockSpawner := setupTestOrchestratorForLogs(t)

	mockSpawner.On("GetLogs", mock.Anything, "JOB-1").Return(
		io.NopCloser(strings.NewReader("line 1\npanic: runtime error\nline 3\n")), nil,
	)
	mockSpawner.On("GetLogs", mock.Anything, "JOB-2").Return(
		io.NopCloser(strings.NewReader("everything is fine\n")), nil,
	)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	req := httptest.NewRequest("GET", "/jobs/search/logs?q=panic", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var results []map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&results)
	assert.NoError(t, err)

	assert.Len(t, results, 1)
	assert.Equal(t, "JOB-1", results[0]["job_id"])
	assert.Equal(t, "Job 1", results[0]["summary"])
	assert.Equal(t, "Completed", results[0]["status"])

	matches := results[0]["matches"].([]interface{})
	assert.Len(t, matches, 1)

	match := matches[0].(map[string]interface{})
	assert.Equal(t, float64(2), match["line_number"])
	assert.Equal(t, "panic: runtime error", match["text"])
}

func TestAPI_SearchLogs_Context(t *testing.T) {
	orch, mockSpawner := setupTestOrchestratorForLogs(t)

	mockSpawner.On("GetLogs", mock.Anything, "JOB-1").Return(
		io.NopCloser(strings.NewReader("line 1\nline 2\npanic: runtime error\nline 4\nline 5\n")), nil,
	)
	mockSpawner.On("GetLogs", mock.Anything, "JOB-2").Return(
		io.NopCloser(strings.NewReader("everything is fine\n")), nil,
	)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	req := httptest.NewRequest("GET", "/jobs/search/logs?q=panic&context=1", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var results []map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&results)
	assert.NoError(t, err)

	assert.Len(t, results, 1)
	assert.Equal(t, "JOB-1", results[0]["job_id"])

	matches := results[0]["matches"].([]interface{})
	assert.Len(t, matches, 1)

	match := matches[0].(map[string]interface{})
	assert.Equal(t, float64(3), match["line_number"])
	assert.Equal(t, "panic: runtime error", match["text"])

	ctxBefore := match["context_before"].([]interface{})
	assert.Len(t, ctxBefore, 1)
	assert.Equal(t, float64(2), ctxBefore[0].(map[string]interface{})["line_number"])
	assert.Equal(t, "line 2", ctxBefore[0].(map[string]interface{})["text"])

	ctxAfter := match["context_after"].([]interface{})
	assert.Len(t, ctxAfter, 1)
	assert.Equal(t, float64(4), ctxAfter[0].(map[string]interface{})["line_number"])
	assert.Equal(t, "line 4", ctxAfter[0].(map[string]interface{})["text"])
}

func TestAPI_SearchLogs_NoMatch(t *testing.T) {
	orch, mockSpawner := setupTestOrchestratorForLogs(t)

	mockSpawner.On("GetLogs", mock.Anything, mock.Anything).Return(
		io.NopCloser(strings.NewReader("nothing to see here\n")), nil,
	)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	req := httptest.NewRequest("GET", "/jobs/search/logs?q=panic", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var results []map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&results)
	assert.NoError(t, err)
	assert.Len(t, results, 0) // or null
}

func TestAPI_SearchLogs_FilterByTag(t *testing.T) {
	orch, mockSpawner := setupTestOrchestratorForLogs(t)

	mockSpawner.On("GetLogs", mock.Anything, "JOB-1").Return(
		io.NopCloser(strings.NewReader("line 1\npanic: runtime error\nline 3\n")), nil,
	)
	// JOB-2 should be filtered out by tag before GetLogs is even called

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	req := httptest.NewRequest("GET", "/jobs/search/logs?q=panic&tag=tag1", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var results []map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&results)
	assert.NoError(t, err)

	assert.Len(t, results, 1)
	assert.Equal(t, "JOB-1", results[0]["job_id"])

	mockSpawner.AssertNotCalled(t, "GetLogs", mock.Anything, "JOB-2")
}

func TestAPI_SearchLogs_FilterByStatus(t *testing.T) {
	orch, mockSpawner := setupTestOrchestratorForLogs(t)

	// JOB-1 will be filtered out because status is 'Failed', and JOB-1 is 'Completed'
	mockSpawner.On("GetLogs", mock.Anything, "JOB-2").Return(
		io.NopCloser(strings.NewReader("line 1\npanic: something failed\n")), nil,
	)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	req := httptest.NewRequest("GET", "/jobs/search/logs?q=panic&status=failed", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var results []map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&results)
	assert.NoError(t, err)

	assert.Len(t, results, 1)
	assert.Equal(t, "JOB-2", results[0]["job_id"])

	mockSpawner.AssertNotCalled(t, "GetLogs", mock.Anything, "JOB-1")
}

func TestAPI_SearchLogs_InvalidRegex(t *testing.T) {
	orch, _ := setupTestOrchestratorForLogs(t)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	req := httptest.NewRequest("GET", "/jobs/search/logs?q=([invalid", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid regex query")
}

func TestAPI_SearchLogs_MissingQuery(t *testing.T) {
	orch, _ := setupTestOrchestratorForLogs(t)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	req := httptest.NewRequest("GET", "/jobs/search/logs", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Query parameter 'q' is required")
}
