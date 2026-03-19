package orchestrator

import (
	"context"
	"encoding/csv"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestOrchestratorForMetricsExport(t *testing.T) *Orchestrator {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	now := time.Now()

	// Add an active job
	orch.activeJobs["JOB-1"] = JobInfo{
		ID:        "JOB-1",
		Status:    "Running",
		StartTime: now.Add(-10 * time.Minute),
		Metrics: map[string]float64{
			"cpu_usage": 15.5,
			"ram_mb":    256.0,
		},
	}

	// Add completed jobs
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:        "JOB-2",
		Status:    "Completed",
		StartTime: now.Add(-20 * time.Minute),
		EndTime:   now.Add(-15 * time.Minute),
		Metrics: map[string]float64{
			"cpu_usage": 80.0,
			"disk_io":   12.3,
		},
	})
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:        "JOB-3",
		Status:    "Failed",
		StartTime: now.Add(-30 * time.Minute),
		EndTime:   now.Add(-25 * time.Minute),
		Metrics: map[string]float64{
			"cpu_usage": 99.9,
			"ram_mb":    1024.0,
		},
	})

	return orch
}

func TestAPI_ExportMetrics_All(t *testing.T) {
	orch := setupTestOrchestratorForMetricsExport(t)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	req := httptest.NewRequest("GET", "/jobs/export/metrics?state=all", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "text/csv", rr.Header().Get("Content-Type"))
	assert.Equal(t, "attachment; filename=metrics_export.csv", rr.Header().Get("Content-Disposition"))

	reader := csv.NewReader(rr.Body)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	// We expect 4 rows: 1 header + 3 jobs
	require.Len(t, records, 4)

	// Validate header (metric keys should be sorted: cpu_usage, disk_io, ram_mb)
	assert.Equal(t, []string{"JobID", "Status", "StartTime", "Duration", "cpu_usage", "disk_io", "ram_mb"}, records[0])

	// Validate JOB-1 (Active)
	assert.Equal(t, "JOB-1", records[1][0])
	assert.Equal(t, "Running", records[1][1])
	assert.Equal(t, "15.5", records[1][4]) // cpu_usage
	assert.Equal(t, "", records[1][5])     // disk_io
	assert.Equal(t, "256", records[1][6])  // ram_mb

	// Validate JOB-2 (Completed)
	assert.Equal(t, "JOB-2", records[2][0])
	assert.Equal(t, "Completed", records[2][1])
	assert.Equal(t, "80", records[2][4])
	assert.Equal(t, "12.3", records[2][5])
	assert.Equal(t, "", records[2][6])

	// Validate JOB-3 (Failed)
	assert.Equal(t, "JOB-3", records[3][0])
	assert.Equal(t, "Failed", records[3][1])
	assert.Equal(t, "99.9", records[3][4])
	assert.Equal(t, "", records[3][5])
	assert.Equal(t, "1024", records[3][6])
}

func TestAPI_ExportMetrics_Active(t *testing.T) {
	orch := setupTestOrchestratorForMetricsExport(t)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	req := httptest.NewRequest("GET", "/jobs/export/metrics?state=active", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	reader := csv.NewReader(rr.Body)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	// 1 header + 1 active job
	require.Len(t, records, 2)
	assert.Equal(t, "JOB-1", records[1][0])
	assert.Equal(t, "Running", records[1][1])
}

func TestAPI_ExportMetrics_Completed(t *testing.T) {
	orch := setupTestOrchestratorForMetricsExport(t)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	req := httptest.NewRequest("GET", "/jobs/export/metrics?state=completed", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	reader := csv.NewReader(rr.Body)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	// 1 header + 1 completed job
	require.Len(t, records, 2)
	assert.Equal(t, "JOB-2", records[1][0])
	assert.Equal(t, "Completed", records[1][1])
}

func TestAPI_ExportMetrics_Failed(t *testing.T) {
	orch := setupTestOrchestratorForMetricsExport(t)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	req := httptest.NewRequest("GET", "/jobs/export/metrics?state=failed", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	reader := csv.NewReader(rr.Body)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	// 1 header + 1 failed job
	require.Len(t, records, 2)
	assert.Equal(t, "JOB-3", records[1][0])
	assert.Equal(t, "Failed", records[1][1])
}
