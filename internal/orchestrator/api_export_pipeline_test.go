package orchestrator

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAPIExportPipeline_Success(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	// Add an active job
	orch.activeJobs["job-1"] = JobInfo{
		ID: "job-1",
		WorkItem: WorkItem{
			Summary: "Task 1",
		},
		Status: "Running",
	}

	// Add a pending job
	orch.pendingJobs["job-2"] = JobInfo{
		ID: "job-2",
		WorkItem: WorkItem{
			Summary:   "Task 2",
			DependsOn: []string{"job-1"},
		},
		Status: "Pending",
	}

	// Add a completed job
	orch.completedJobs = []JobInfo{
		{
			ID: "job-0",
			WorkItem: WorkItem{
				Summary: "Completed Task",
			},
			Status: "Completed",
		},
	}

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	// Test default state (active + pending)
	resp, err := http.Get(server.URL + "/jobs/export/pipeline?name=test-pipe")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/x-yaml", resp.Header.Get("Content-Type"))
	assert.Equal(t, "attachment; filename=test-pipe.yaml", resp.Header.Get("Content-Disposition"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var p Pipeline
	err = yaml.Unmarshal(body, &p)
	require.NoError(t, err)

	assert.Equal(t, "test-pipe", p.Name)
	assert.Len(t, p.Jobs, 2)
	assert.Contains(t, p.Jobs, "job-1")
	assert.Contains(t, p.Jobs, "job-2")
	assert.NotContains(t, p.Jobs, "job-0")

	// Test state=all
	respAll, err := http.Get(server.URL + "/jobs/export/pipeline?name=all-pipe&state=all")
	require.NoError(t, err)
	defer respAll.Body.Close()

	assert.Equal(t, http.StatusOK, respAll.StatusCode)
	bodyAll, _ := io.ReadAll(respAll.Body)

	var pAll Pipeline
	yaml.Unmarshal(bodyAll, &pAll)
	assert.Equal(t, "all-pipe", pAll.Name)
	assert.Len(t, pAll.Jobs, 3)
	assert.Contains(t, pAll.Jobs, "job-0")
	assert.Contains(t, pAll.Jobs, "job-1")
	assert.Contains(t, pAll.Jobs, "job-2")

	// Test state=completed
	respCompleted, err := http.Get(server.URL + "/jobs/export/pipeline?state=completed")
	require.NoError(t, err)
	defer respCompleted.Body.Close()
	assert.Equal(t, http.StatusOK, respCompleted.StatusCode)
	bodyCompleted, _ := io.ReadAll(respCompleted.Body)

	var pComp Pipeline
	yaml.Unmarshal(bodyCompleted, &pComp)
	assert.Equal(t, "exported-pipeline", pComp.Name) // Default name
	assert.Len(t, pComp.Jobs, 1)
	assert.Contains(t, pComp.Jobs, "job-0")
}
