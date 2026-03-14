package orchestrator

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAPI_RenameJob(t *testing.T) {
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

	// Call API to Rename Job
	reqBody := `{"new_id": "NEW-J1"}`
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/jobs/J1/rename", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	pending := orch.GetPendingJobs()
	assert.Len(t, pending, 1)
	assert.Equal(t, "NEW-J1", pending[0].ID)

	// Call API to Rename Non-existent Job
	reqBody = `{"new_id": "SOME-ID"}`
	req, _ = http.NewRequest(http.MethodPut, server.URL+"/jobs/NON-EXISTENT/rename", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Call API with empty body
	req, _ = http.NewRequest(http.MethodPut, server.URL+"/jobs/NEW-J1/rename", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Call API with empty new_id
	reqBody = `{"new_id": ""}`
	req, _ = http.NewRequest(http.MethodPut, server.URL+"/jobs/NEW-J1/rename", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
