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
	"github.com/stretchr/testify/require"
)

func TestAPI_UpdateJobTags(t *testing.T) {
	orch := New(nil, nil, time.Minute)

	// Setup a pending job
	job := JobInfo{
		ID:     "JOB-1",
		Status: "Pending",
		WorkItem: WorkItem{
			ID: "JOB-1",
		},
	}
	orch.pendingJobs["JOB-1"] = job

	// Setup an active job
	activeJob := JobInfo{
		ID:     "JOB-2",
		Status: "Running",
	}
	orch.activeJobs["JOB-2"] = activeJob

	// Setup a completed job
	completedJob := JobInfo{
		ID:     "JOB-3",
		Status: "Completed",
	}
	orch.completedJobs = append(orch.completedJobs, completedJob)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Run("Success_UpdateTags", func(t *testing.T) {
		payload := map[string]interface{}{
			"tags": []string{"urgent", "backend"},
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, server.URL+"/jobs/JOB-1/tags", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify tags were updated
		updatedJob, ok := orch.pendingJobs["JOB-1"]
		require.True(t, ok)
		assert.Equal(t, []string{"urgent", "backend"}, updatedJob.WorkItem.Tags)
	})

	t.Run("Error_ActiveJob", func(t *testing.T) {
		payload := map[string]interface{}{
			"tags": []string{"feature"},
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, server.URL+"/jobs/JOB-2/tags", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	t.Run("Error_CompletedJob", func(t *testing.T) {
		payload := map[string]interface{}{
			"tags": []string{"feature"},
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, server.URL+"/jobs/JOB-3/tags", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	t.Run("Error_NotFound", func(t *testing.T) {
		payload := map[string]interface{}{
			"tags": []string{"feature"},
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, server.URL+"/jobs/NON-EXISTENT/tags", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("Error_InvalidJSON", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPut, server.URL+"/jobs/JOB-1/tags", bytes.NewBufferString("invalid"))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestAPI_UpdateBulkJobTags(t *testing.T) {
	orch := New(nil, nil, time.Minute)

	orch.pendingJobs["JOB-1"] = JobInfo{
		ID:     "JOB-1",
		Status: "Pending",
		WorkItem: WorkItem{
			ID:   "JOB-1",
			Tags: []string{"old-tag"},
		},
	}
	orch.pendingJobs["JOB-2"] = JobInfo{
		ID:      "JOB-2",
		Status:  "Pending",
		Summary: "Match me",
		WorkItem: WorkItem{
			ID:   "JOB-2",
			Tags: []string{"other-tag"},
		},
	}

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Run("Success_UpdateBulkTagsByTag", func(t *testing.T) {
		payload := map[string]interface{}{
			"tags": []string{"new-tag"},
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, server.URL+"/jobs/tags?tag=old-tag", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]int
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, 1, result["updated"])

		updatedJob, ok := orch.pendingJobs["JOB-1"]
		require.True(t, ok)
		assert.Equal(t, []string{"new-tag"}, updatedJob.WorkItem.Tags)
	})

	t.Run("Success_UpdateBulkTagsByMatch", func(t *testing.T) {
		payload := map[string]interface{}{
			"tags": []string{"matched-tag"},
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, server.URL+"/jobs/tags?match=Match", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]int
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, 1, result["updated"])

		updatedJob, ok := orch.pendingJobs["JOB-2"]
		require.True(t, ok)
		assert.Equal(t, []string{"matched-tag"}, updatedJob.WorkItem.Tags)
	})

	t.Run("Error_MissingQueryParam", func(t *testing.T) {
		payload := map[string]interface{}{
			"tags": []string{"new-tag"},
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, server.URL+"/jobs/tags", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Error_BothQueryParams", func(t *testing.T) {
		payload := map[string]interface{}{
			"tags": []string{"new-tag"},
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, server.URL+"/jobs/tags?tag=old-tag&match=Match", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
