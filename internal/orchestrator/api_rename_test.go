package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAPI_RenameJob(t *testing.T) {
	// Setup orchestrator
	poller := new(MockPoller)
	spawner := new(MockSpawner)
	orch := New(poller, spawner, 1*time.Second)

	// Add a pending job
	job := WorkItem{
		ID:        "api-old-id",
		Summary:   "API Rename Test",
		DependsOn: []string{"unmet-dep"},
	}
	err := orch.SubmitJob(context.Background(), job, nil)
	require.NoError(t, err)

	// Setup HTTP server
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	// 1. Rename successfully
	renameReq := map[string]string{"new_id": "api-new-id"}
	body, _ := json.Marshal(renameReq)

	req, err := http.NewRequest(http.MethodPut, server.URL+"/jobs/api-old-id/rename", bytes.NewReader(body))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify job was renamed
	_, err = orch.GetJob("api-old-id")
	require.ErrorContains(t, err, "not found")

	j, err := orch.GetJob("api-new-id")
	require.NoError(t, err)
	require.Equal(t, "api-new-id", j.ID)

	// 2. Try renaming a missing job
	renameReq2 := map[string]string{"new_id": "new-missing-id"}
	body2, _ := json.Marshal(renameReq2)
	req2, _ := http.NewRequest(http.MethodPut, server.URL+"/jobs/missing-id/rename", bytes.NewReader(body2))
	resp2, _ := http.DefaultClient.Do(req2)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusNotFound, resp2.StatusCode)

	// 3. Try renaming to an existing job
	orch.SubmitJob(context.Background(), WorkItem{ID: "another-api-id", DependsOn: []string{"unmet"}}, nil)
	renameReq3 := map[string]string{"new_id": "api-new-id"} // already exists
	body3, _ := json.Marshal(renameReq3)

	req3, _ := http.NewRequest(http.MethodPut, server.URL+"/jobs/another-api-id/rename", bytes.NewReader(body3))
	resp3, _ := http.DefaultClient.Do(req3)
	defer resp3.Body.Close()
	require.Equal(t, http.StatusConflict, resp3.StatusCode)
}
