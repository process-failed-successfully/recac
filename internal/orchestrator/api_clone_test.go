package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

import (
	"log/slog"
	"os"
)

func TestAPI_CloneJob(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)

	orch := New(mockPoller, mockSpawner, 1*time.Minute)
	// Add an existing job to history
	originalItem := WorkItem{
		ID:       "orig-123",
		Summary:  "Original Job",
		Priority: 10,
		EnvVars: map[string]string{
			"VAR1": "value1",
		},
		DependsOn: []string{"dep-1"},
	}
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:       "orig-123",
		Status:   "Failed",
		WorkItem: originalItem,
	})

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Run("Clone without overrides", func(t *testing.T) {
		// Expect the Spawner to be called with the cloned item
		mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
			return item.Summary == "Original Job" &&
				item.Priority == 10 &&
				item.EnvVars["VAR1"] == "value1" &&
				len(item.DependsOn) == 1 && item.DependsOn[0] == "dep-1"
		})).Return(nil).Once()

		url := server.URL + "/jobs/orig-123/clone"
		req, err := http.NewRequest(http.MethodPost, url, nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		var result map[string]string
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Contains(t, result["cloned_job_id"], "orig-123-clone-")
	})

	t.Run("Clone with overrides", func(t *testing.T) {
		p := 20
		overrides := map[string]interface{}{
			"new_id": "new-clone-id",
			"env_vars": map[string]string{
				"VAR2": "value2",
			},
			"priority":   &p,
			"depends_on": []string{"dep-2"},
		}

		payload, _ := json.Marshal(overrides)

		mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
			return item.ID == "new-clone-id" &&
				item.Summary == "Original Job" &&
				item.Priority == 20 &&
				item.EnvVars["VAR1"] == "value1" && // Original var is kept
				item.EnvVars["VAR2"] == "value2" && // New var is added
				len(item.DependsOn) == 1 && item.DependsOn[0] == "dep-2" // DependsOn is overridden
		})).Return(nil).Once()

		url := server.URL + "/jobs/orig-123/clone"
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		var result map[string]string
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "new-clone-id", result["cloned_job_id"])

		// Assert that the original job was NOT modified
		assert.Equal(t, "orig-123", orch.completedJobs[0].WorkItem.ID)
		assert.Equal(t, "Original Job", orch.completedJobs[0].WorkItem.Summary)
		assert.Equal(t, 10, orch.completedJobs[0].WorkItem.Priority)
		assert.Len(t, orch.completedJobs[0].WorkItem.EnvVars, 1)
		assert.Equal(t, "value1", orch.completedJobs[0].WorkItem.EnvVars["VAR1"])
		assert.Len(t, orch.completedJobs[0].WorkItem.DependsOn, 1)
		assert.Equal(t, "dep-1", orch.completedJobs[0].WorkItem.DependsOn[0])
	})

	t.Run("Clone non-existent job", func(t *testing.T) {
		url := server.URL + "/jobs/not-found/clone"
		req, err := http.NewRequest(http.MethodPost, url, nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
