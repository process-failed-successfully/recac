package orchestrator

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAPI_ExportGraph(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	orch := New(mockPoller, mockSpawner, 1*time.Minute)

	orch.activeJobs = map[string]JobInfo{
		"job-1": {ID: "job-1", Status: "Running", WorkItem: WorkItem{DependsOn: []string{}}},
	}
	orch.pendingJobs = map[string]JobInfo{
		"job-2": {ID: "job-2", Status: "Pending", WorkItem: WorkItem{DependsOn: []string{"job-1"}}},
	}
	orch.completedJobs = []JobInfo{
		{ID: "job-0", Status: "Completed", WorkItem: WorkItem{}},
	}

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Run("Default Mermaid", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/jobs/export/graph")
		assert.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))

		body, _ := io.ReadAll(resp.Body)
		out := string(body)
		assert.Contains(t, out, "graph TD;")
		assert.Contains(t, out, "job_1")
		assert.Contains(t, out, "job_2")
		assert.Contains(t, out, "job_0")
		assert.Contains(t, out, "job_1 --> job_2")
	})

	t.Run("DOT Format", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/jobs/export/graph?format=dot")
		assert.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/vnd.graphviz", resp.Header.Get("Content-Type"))

		body, _ := io.ReadAll(resp.Body)
		out := string(body)
		assert.Contains(t, out, "digraph G {")
		assert.Contains(t, out, "\"job_1\"")
		assert.Contains(t, out, "\"job_2\"")
		assert.Contains(t, out, "\"job_0\"")
		assert.Contains(t, out, "\"job_1\" -> \"job_2\"")
	})

	t.Run("Invalid Format", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/jobs/export/graph?format=unknown")
		assert.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Filter by State", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/jobs/export/graph?state=completed")
		assert.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		out := string(body)
		assert.Contains(t, out, "job_0")
		assert.NotContains(t, out, "job_1")
	})
}
