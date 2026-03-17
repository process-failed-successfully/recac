package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupBulkCloneTest(t *testing.T) (*Orchestrator, *MockSpawner, *httptest.Server) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)

	orch := New(mockPoller, mockSpawner, 1*time.Minute)

	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:      "job-1",
		Status:  "Completed",
		Summary: "Task 1",
		WorkItem: WorkItem{
			ID:       "job-1",
			Summary:  "Task 1",
			Tags:     []string{"bulk-tag", "other-tag"},
			Priority: 5,
			EnvVars: map[string]string{
				"V1": "1",
			},
		},
	})

	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:      "job-2",
		Status:  "Failed",
		Summary: "Task 2 error",
		WorkItem: WorkItem{
			ID:       "job-2",
			Summary:  "Task 2 error",
			Tags:     []string{"bulk-tag"},
			Priority: 5,
		},
	})

	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:      "job-3",
		Status:  "Completed",
		Summary: "Another task",
		WorkItem: WorkItem{
			ID:      "job-3",
			Summary: "Another task",
			Tags:    []string{"ignore-tag"},
		},
	})

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())
	server := httptest.NewServer(mux)

	return orch, mockSpawner, server
}

func TestAPI_CloneBulk_Tag(t *testing.T) {
	_, mockSpawner, server := setupBulkCloneTest(t)
	defer server.Close()

	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil).Twice()

	url := server.URL + "/jobs/clone/bulk?tag=bulk-tag"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, float64(2), result["cloned"])
	clonedIDs := result["cloned_job_ids"].([]interface{})
	assert.Len(t, clonedIDs, 2)
}

func TestAPI_CloneBulk_Match(t *testing.T) {
	_, mockSpawner, server := setupBulkCloneTest(t)
	defer server.Close()

	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil).Once()

	url := server.URL + "/jobs/clone/bulk?match=Task%202"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, float64(1), result["cloned"])
	clonedIDs := result["cloned_job_ids"].([]interface{})
	assert.Len(t, clonedIDs, 1)
}

func TestAPI_CloneBulk_Overrides(t *testing.T) {
	_, mockSpawner, server := setupBulkCloneTest(t)
	defer server.Close()

	p := 10
	overrides := map[string]interface{}{
		"env_vars": map[string]string{
			"V2": "2",
		},
		"priority":   &p,
		"depends_on": []string{"dep-new"},
	}
	payload, _ := json.Marshal(overrides)

	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.Priority == 10 &&
			item.EnvVars["V2"] == "2" &&
			len(item.DependsOn) == 1 && item.DependsOn[0] == "dep-new"
	})).Return(nil).Twice()

	url := server.URL + "/jobs/clone/bulk?tag=bulk-tag"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, float64(2), result["cloned"])
}

func TestAPI_CloneBulk_MissingParams(t *testing.T) {
	_, _, server := setupBulkCloneTest(t)
	defer server.Close()

	url := server.URL + "/jobs/clone/bulk"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAPI_CloneBulkJobs_RemapDeps(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)

	orch := New(mockPoller, mockSpawner, 1*time.Minute)

	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:      "job-a",
		Status:  "Completed",
		Summary: "Task A",
		WorkItem: WorkItem{
			ID:       "job-a",
			Summary:  "Task A",
			Tags:     []string{"pipeline-1"},
		},
	})

	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:      "job-b",
		Status:  "Completed",
		Summary: "Task B",
		WorkItem: WorkItem{
			ID:       "job-b",
			Summary:  "Task B",
			Tags:     []string{"pipeline-1"},
			DependsOn: []string{"job-a"},
		},
	})

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	// Capture the spawned items
	var spawnedItems []WorkItem
	var mu sync.Mutex
	mockSpawner.On("Spawn", mock.Anything, mock.AnythingOfType("orchestrator.WorkItem")).Run(func(args mock.Arguments) {
		item := args.Get(1).(WorkItem)
		mu.Lock()
		spawnedItems = append(spawnedItems, item)
		mu.Unlock()
	}).Return(nil).Twice()

	overrides := map[string]interface{}{
		"remap_dependencies": true,
	}
	payload, _ := json.Marshal(overrides)

	url := server.URL + "/jobs/clone/bulk?tag=pipeline-1"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, float64(2), result["cloned"])

	// Wait for spawns to complete
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Find the new IDs for A and B
	var newJobA_ID, newJobB_ID string
	for _, item := range spawnedItems {
		if item.Summary == "Task A" {
			newJobA_ID = item.ID
		} else if item.Summary == "Task B" {
			newJobB_ID = item.ID
			// Check that the new B depends on the new A
			assert.Len(t, item.DependsOn, 1)
			if len(item.DependsOn) == 1 {
				assert.NotEqual(t, "job-a", item.DependsOn[0], "Dependency should be remapped from job-a")
				// We don't know newJobA_ID yet if B was processed first, but we can verify after the loop
			}
		}
	}

	assert.NotEmpty(t, newJobA_ID)
	assert.NotEmpty(t, newJobB_ID)

	for _, item := range spawnedItems {
		if item.ID == newJobB_ID {
			assert.Equal(t, newJobA_ID, item.DependsOn[0], "Task B should depend on the new Task A ID")
		}
	}
}

func TestAPI_CloneBulk_NoMatches(t *testing.T) {
	_, _, server := setupBulkCloneTest(t)
	defer server.Close()

	url := server.URL + "/jobs/clone/bulk?tag=non-existent-tag"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, float64(0), result["cloned"])
	assert.Empty(t, result["cloned_job_ids"])
}
