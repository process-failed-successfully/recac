package orchestrator

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleSimulate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(ioutil.Discard, nil))
	o := &Orchestrator{
		activeJobs:        make(map[string]JobInfo),
		pendingJobs:       make(map[string]JobInfo),
		completedJobs:     []JobInfo{},
		MaxConcurrentJobs: 2,
	}

	handler := handleSimulate(o, logger)

	t.Run("GET Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/simulate", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		res := w.Result()
		defer res.Body.Close()

		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Equal(t, "application/json", res.Header.Get("Content-Type"))

		var report SimulationReport
		err := json.NewDecoder(res.Body).Decode(&report)
		require.NoError(t, err)

		assert.Equal(t, 0.0, report.EstimatedTotalTimeMs)
		assert.Equal(t, 0, report.JobsProcessed)
	})

	t.Run("POST Method Not Allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/simulate", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		res := w.Result()
		defer res.Body.Close()

		assert.Equal(t, http.StatusMethodNotAllowed, res.StatusCode)
	})
}

func TestHandleSimulatePipeline(t *testing.T) {
	orch := New(nil, nil, 1*time.Second)

	// Add historical data
	t1 := time.Now()
	orch.completedJobs = []JobInfo{
		{
			ID: "hist-1",
			WorkItem: WorkItem{
				Summary: "Build application",
				Tags:    []string{"build"},
			},
			StartTime: t1,
			EndTime:   t1.Add(60 * time.Second),
		},
		{
			ID: "hist-2",
			WorkItem: WorkItem{
				Summary: "Run tests",
				Tags:    []string{"test"},
			},
			StartTime: t1,
			EndTime:   t1.Add(30 * time.Second),
		},
	}

	yamlPayload := []byte(`
name: Pipeline API Test
jobs:
  build:
    summary: Build application
    tags: [build]
  test:
    summary: Run tests
    depends_on: [build]
    tags: [test]
`)

	req := httptest.NewRequest(http.MethodPost, "/simulate/pipeline", bytes.NewBuffer(yamlPayload))
	w := httptest.NewRecorder()

	handler := handleSimulatePipeline(orch, slog.Default())
	handler.ServeHTTP(w, req)

	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)

	var report SimulationReport
	err := json.NewDecoder(res.Body).Decode(&report)
	assert.NoError(t, err)

	// 60s + 30s = 90s = 90000ms
	assert.Equal(t, float64(90000), report.EstimatedTotalTimeMs)
	assert.Equal(t, 2, report.JobsProcessed)
}
