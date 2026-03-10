package orchestrator

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAPI_UpdateDependencies(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Submit a pending job
	job := WorkItem{ID: "JOB-1", DependsOn: []string{"NON-EXISTENT"}}
	orch.SubmitJob(ctx, job, nil)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, ctx)

	t.Run("Success", func(t *testing.T) {
		payload := `{"depends_on": ["NEW-DEP-1"]}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/JOB-1/dependencies", bytes.NewBufferString(payload))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		jobInfo, err := orch.GetJob("JOB-1")
		assert.NoError(t, err)
		assert.Equal(t, []string{"NEW-DEP-1"}, jobInfo.WorkItem.DependsOn)
	})

	t.Run("NotFound", func(t *testing.T) {
		payload := `{"depends_on": ["NEW-DEP-1"]}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/UNKNOWN/dependencies", bytes.NewBufferString(payload))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("ActiveJob", func(t *testing.T) {
		orch.SubmitJob(ctx, WorkItem{ID: "ACTIVE-1"}, nil)
		payload := `{"depends_on": ["NEW-DEP-1"]}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/ACTIVE-1/dependencies", bytes.NewBufferString(payload))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusConflict, rr.Code)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/jobs/JOB-1/dependencies", bytes.NewBufferString("invalid"))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}
