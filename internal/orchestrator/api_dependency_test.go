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

func TestAPI_UpdateDependenciesBulk(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	orch.SubmitJob(ctx, WorkItem{ID: "JOB-A", Tags: []string{"tag1"}, Summary: "Match this summary", DependsOn: []string{"OLD-1"}}, nil)
	orch.SubmitJob(ctx, WorkItem{ID: "JOB-B", Tags: []string{"tag2"}, Summary: "Another summary", DependsOn: []string{"OLD-1"}}, nil)
	orch.SubmitJob(ctx, WorkItem{ID: "JOB-C", Tags: []string{"tag1"}, Summary: "Match this too", DependsOn: []string{"OLD-1"}}, nil)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, ctx)

	t.Run("ByTag", func(t *testing.T) {
		payload := `{"depends_on": ["NEW-DEP-TAG"]}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/dependencies?tag=tag1", bytes.NewBufferString(payload))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, `{"updated": 2}`, rr.Body.String())

		jobA, _ := orch.GetJob("JOB-A")
		assert.Equal(t, []string{"NEW-DEP-TAG"}, jobA.WorkItem.DependsOn)

		jobB, _ := orch.GetJob("JOB-B")
		assert.Equal(t, []string{"OLD-1"}, jobB.WorkItem.DependsOn) // Unchanged

		jobC, _ := orch.GetJob("JOB-C")
		assert.Equal(t, []string{"NEW-DEP-TAG"}, jobC.WorkItem.DependsOn)
	})

	t.Run("ByMatch", func(t *testing.T) {
		payload := `{"depends_on": ["NEW-DEP-MATCH"]}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/dependencies?match=match%20this", bytes.NewBufferString(payload))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, `{"updated": 2}`, rr.Body.String())

		jobA, _ := orch.GetJob("JOB-A")
		assert.Equal(t, []string{"NEW-DEP-MATCH"}, jobA.WorkItem.DependsOn)

		jobB, _ := orch.GetJob("JOB-B")
		assert.Equal(t, []string{"OLD-1"}, jobB.WorkItem.DependsOn) // Unchanged

		jobC, _ := orch.GetJob("JOB-C")
		assert.Equal(t, []string{"NEW-DEP-MATCH"}, jobC.WorkItem.DependsOn)
	})

	t.Run("MissingParams", func(t *testing.T) {
		payload := `{"depends_on": ["NEW-DEP"]}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/dependencies", bytes.NewBufferString(payload))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Either 'tag' or 'match' query parameter is required")
	})
}
