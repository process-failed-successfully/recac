package orchestrator

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"recac/internal/telemetry"
)

func TestHandleAnalyzeAnomalies_Success(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	orch := New(mockPoller, mockSpawner, 1*time.Second)
	logger := telemetry.NewLogger(false, "test", false)

	// Add normal jobs
	for i := 0; i < 5; i++ {
		orch.completedJobs = append(orch.completedJobs, JobInfo{
			ID:        "job-normal-" + string(rune(i)),
			Status:    "Completed",
			StartTime: time.Now().Add(-10 * time.Second),
			EndTime:   time.Now(),
			Metrics: map[string]float64{
				"total_cost": 0.01,
			},
			WorkItem: WorkItem{
				AgentModel: "gpt-4o",
			},
		})
	}

	// Add an anomaly duration job
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:        "job-anomaly-dur",
		Status:    "Failed",
		StartTime: time.Now().Add(-100 * time.Second), // Long duration
		EndTime:   time.Now(),
		Metrics: map[string]float64{
			"total_cost": 0.01,
		},
		WorkItem: WorkItem{
			AgentModel: "gpt-4o",
		},
	})

	// Add an anomaly cost job
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:        "job-anomaly-cost",
		Status:    "Completed",
		StartTime: time.Now().Add(-10 * time.Second),
		EndTime:   time.Now(),
		Metrics: map[string]float64{
			"total_cost": 5.00, // High cost
		},
		WorkItem: WorkItem{
			AgentModel: "gpt-4o",
		},
	})

	handler := handleAnalyzeAnomalies(orch, logger)

	req := httptest.NewRequest(http.MethodGet, "/jobs/analyze/anomalies", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var anomalies []AnomalyReport
	err := json.NewDecoder(rr.Body).Decode(&anomalies)
	require.NoError(t, err)

	assert.Len(t, anomalies, 2)
	assert.Contains(t, []string{"job-anomaly-dur", "job-anomaly-cost"}, anomalies[0].JobID)
	assert.Contains(t, []string{"job-anomaly-dur", "job-anomaly-cost"}, anomalies[1].JobID)
}

func TestHandleAnalyzeAnomalies_MethodNotAllowed(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	orch := New(mockPoller, mockSpawner, 1*time.Second)
	logger := telemetry.NewLogger(false, "test", false)

	handler := handleAnalyzeAnomalies(orch, logger)

	req := httptest.NewRequest(http.MethodPost, "/jobs/analyze/anomalies", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleAnalyzeAnomalies_Empty(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	orch := New(mockPoller, mockSpawner, 1*time.Second)
	logger := telemetry.NewLogger(false, "test", false)

	handler := handleAnalyzeAnomalies(orch, logger)

	req := httptest.NewRequest(http.MethodGet, "/jobs/analyze/anomalies", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var anomalies []AnomalyReport
	err := json.NewDecoder(rr.Body).Decode(&anomalies)
	require.NoError(t, err)
	assert.Empty(t, anomalies)
}
