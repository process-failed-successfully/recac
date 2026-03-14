package orchestrator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAPI_SkipJob(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)
	orch.RequireApproval = true
	ctx := context.Background()

	orch.SubmitJob(ctx, WorkItem{ID: "J1", Summary: "S1"}, nil)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, ctx)

	server := httptest.NewServer(mux)
	defer server.Close()

	// Call API to Skip Job
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/jobs/J1/skip", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 1)
	assert.Equal(t, "J1", completed[0].ID)
	assert.Equal(t, "Skipped", completed[0].Status)

	// Call API to Skip Non-existent Job
	req, _ = http.NewRequest(http.MethodPost, server.URL+"/jobs/NON-EXISTENT/skip", nil)
	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestAPI_BulkSkip(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)
	orch.RequireApproval = true
	ctx := context.Background()

	orch.SubmitJob(ctx, WorkItem{ID: "J1", Summary: "S1", Tags: []string{"tag1"}}, nil)
	orch.SubmitJob(ctx, WorkItem{ID: "J2", Summary: "S2", Tags: []string{"tag1", "tag2"}}, nil)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, ctx)

	server := httptest.NewServer(mux)
	defer server.Close()

	// Call Bulk Skip by Tag
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/jobs/skip?tag=tag1", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 2)
	assert.Equal(t, "Skipped", completed[0].Status)
	assert.Equal(t, "Skipped", completed[1].Status)
}

func TestAPI_BulkSkip_InvalidRegex(t *testing.T) {
	orch := New(&MockPoller{}, &MockSpawner{}, 1*time.Minute)
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	// Call Bulk Skip by Invalid Match
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/jobs/skip?match=[invalid", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAPI_BulkSkip_MissingParam(t *testing.T) {
	orch := New(&MockPoller{}, &MockSpawner{}, 1*time.Minute)
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	// Call Bulk Skip without Tag or Match
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/jobs/skip", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
