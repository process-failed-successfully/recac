package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPI_HealBulk(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)

	// Add failed job to history
	job1 := JobInfo{
		ID:      "JOB-HEAL-1",
		Status:  "Failed",
		WorkItem: WorkItem{
			ID:   "JOB-HEAL-1",
			Tags: []string{"broken-tag"},
		},
	}
	orch.addToHistory(job1, nil)

	mux := http.NewServeMux()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	RegisterAPI(mux, orch, logger, context.Background())

	server := httptest.NewServer(mux)
	defer server.Close()

	// Test 1: Successful Heal by Tag
	req, _ := http.NewRequest("POST", server.URL+"/jobs/heal/bulk?tag=broken-tag", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]int
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, 1, result["healed"])

	// Test 2: Missing match and tag
	reqMissing, _ := http.NewRequest("POST", server.URL+"/jobs/heal/bulk", nil)
	respMissing, err := http.DefaultClient.Do(reqMissing)
	require.NoError(t, err)
	defer respMissing.Body.Close()

	assert.Equal(t, http.StatusBadRequest, respMissing.StatusCode)
}
