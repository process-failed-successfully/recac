package orchestrator

import (
	"encoding/json"
	"io"
	"log/slog"
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

type failingResponseWriter struct {
	http.ResponseWriter
}

func (f *failingResponseWriter) Write(b []byte) (int, error) {
	return 0, assert.AnError
}

func TestHandleDiagnose_EncodeError(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := handleDiagnose(orch, logger)
	req := httptest.NewRequest(http.MethodGet, "/diagnose", nil)
	rr := httptest.NewRecorder()

	failingWriter := &failingResponseWriter{rr}

	// It logs an error and calls http.Error which will also fail to write, but we hit the branch.
	// However, json.NewEncoder does not fail on Write immediately if the data is small enough, it buffers.
	// But for a failing writer that returns error immediately on any write, it will return an error from Encode.
	handler.ServeHTTP(failingWriter, req)
}
