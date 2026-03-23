package orchestrator

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHandleDiagnose(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	// Add missing dependency
	orch.pendingJobs["job-1"] = JobInfo{
		ID:     "job-1",
		Status: "Pending",
		WorkItem: WorkItem{
			ID:        "job-1",
			DependsOn: []string{"job-missing"},
		},
	}

	handler := handleDiagnose(orch, nil)
	req := httptest.NewRequest(http.MethodGet, "/diagnose", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var report DiagnosticReport
	err := json.NewDecoder(rr.Body).Decode(&report)
	assert.NoError(t, err)

	assert.Len(t, report.UnresolvableJobs, 1)
	assert.Equal(t, "job-1", report.UnresolvableJobs[0].JobID)
	assert.Contains(t, report.UnresolvableJobs[0].MissingDeps, "job-missing")
}

func TestHandleDiagnose_MethodNotAllowed(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	handler := handleDiagnose(orch, nil)
	req := httptest.NewRequest(http.MethodPost, "/diagnose", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}
