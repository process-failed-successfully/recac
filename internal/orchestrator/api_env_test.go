package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"recac/internal/telemetry"
)

func TestAPI_UpdateJobEnv(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := new(MockSpawner)
	orch := New(poller, spawner, 1*time.Minute)
	orch.RequireApproval = true

	orch.mu.Lock()
	orch.pendingJobs["JOB-1"] = JobInfo{
		ID:       "JOB-1",
		Status:   "Pending Approval",
		WorkItem: WorkItem{ID: "JOB-1"},
	}
	orch.mu.Unlock()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, nil)
	server := httptest.NewServer(mux)
	defer server.Close()

	reqBody := struct {
		EnvVars map[string]string `json:"env_vars"`
	}{
		EnvVars: map[string]string{"NEW_ENV": "VALUE123"},
	}

	payload, _ := json.Marshal(reqBody)

	req, err := http.NewRequest(http.MethodPut, server.URL+"/jobs/JOB-1/env", bytes.NewReader(payload))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	orch.mu.Lock()
	job := orch.pendingJobs["JOB-1"]
	orch.mu.Unlock()

	assert.Equal(t, "VALUE123", job.WorkItem.EnvVars["NEW_ENV"])
}

func TestAPIEnvBulk(t *testing.T) {
	orch := New(newMockPoller(nil), new(MockSpawner), time.Second)
	ctx := context.Background()
	logger := telemetry.NewLogger(false, "test", false)

	// Create a job pending approval so it stays in pending queue
	orch.RequireApproval = true
	job1 := WorkItem{ID: "JOB-1", Summary: "Update backend", Tags: []string{"backend"}}
	job2 := WorkItem{ID: "JOB-2", Summary: "Fix frontend", Tags: []string{"frontend"}}
	require.NoError(t, orch.SubmitJob(ctx, job1, logger))
	require.NoError(t, orch.SubmitJob(ctx, job2, logger))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux := http.NewServeMux()
		RegisterAPI(mux, orch, logger, ctx)
		mux.ServeHTTP(w, r)
	}))
	defer server.Close()

	// 1. Valid Tag update
	reqBody := `{"env_vars": {"ENV_1": "VAL_1"}}`
	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/jobs/env?tag=backend", server.URL), bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var res map[string]int
	json.NewDecoder(resp.Body).Decode(&res)
	require.Equal(t, 1, res["updated"])

	job1Fetched, _ := orch.GetJob("JOB-1")
	require.Equal(t, "VAL_1", job1Fetched.WorkItem.EnvVars["ENV_1"])
	job2Fetched, _ := orch.GetJob("JOB-2")
	require.Empty(t, job2Fetched.WorkItem.EnvVars["ENV_1"])

	// 2. Valid Match update
	reqBodyMatch := `{"env_vars": {"ENV_2": "VAL_2"}}`
	reqMatch, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/jobs/env?match=Fix", server.URL), bytes.NewBufferString(reqBodyMatch))
	reqMatch.Header.Set("Content-Type", "application/json")
	respMatch, errMatch := http.DefaultClient.Do(reqMatch)
	require.NoError(t, errMatch)
	require.Equal(t, http.StatusOK, respMatch.StatusCode)

	var resMatch map[string]int
	json.NewDecoder(respMatch.Body).Decode(&resMatch)
	require.Equal(t, 1, resMatch["updated"])

	job2FetchedMatch, _ := orch.GetJob("JOB-2")
	require.Equal(t, "VAL_2", job2FetchedMatch.WorkItem.EnvVars["ENV_2"])

	// 3. Mutually exclusive parameters
	reqBoth, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/jobs/env?match=Fix&tag=backend", server.URL), bytes.NewBufferString(reqBodyMatch))
	reqBoth.Header.Set("Content-Type", "application/json")
	respBoth, _ := http.DefaultClient.Do(reqBoth)
	require.Equal(t, http.StatusBadRequest, respBoth.StatusCode)

	// 4. Missing parameters
	reqMissing, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/jobs/env", server.URL), bytes.NewBufferString(reqBodyMatch))
	reqMissing.Header.Set("Content-Type", "application/json")
	respMissing, _ := http.DefaultClient.Do(reqMissing)
	require.Equal(t, http.StatusBadRequest, respMissing.StatusCode)

	// 5. Empty Payload
	reqEmpty, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/jobs/env?tag=backend", server.URL), bytes.NewBufferString(""))
	reqEmpty.Header.Set("Content-Type", "application/json")
	respEmpty, _ := http.DefaultClient.Do(reqEmpty)
	require.Equal(t, http.StatusBadRequest, respEmpty.StatusCode)

	// 6. Invalid regex match
	reqInvalid, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/jobs/env?match=[", server.URL), bytes.NewBufferString(reqBodyMatch))
	reqInvalid.Header.Set("Content-Type", "application/json")
	respInvalid, _ := http.DefaultClient.Do(reqInvalid)
	require.Equal(t, http.StatusBadRequest, respInvalid.StatusCode)
}
