package orchestrator

import (
	"encoding/json"
	"io/ioutil"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

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
