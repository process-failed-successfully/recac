package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"recac/internal/orchestrator"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLimitString(t *testing.T) {
	assert.Equal(t, "short", limitString("short", 10))
	assert.Equal(t, "exactlen", limitString("exactlen", 8))
	assert.Equal(t, "longst...", limitString("longstring", 6))
}

func TestFetchStatus_Success(t *testing.T) {
	// Setup Mock Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			status := orchestrator.Status{
				Uptime:       "1h",
				ActiveSpawns: 5,
			}
			json.NewEncoder(w).Encode(status)
			return
		}
		if r.URL.Path == "/jobs" {
			jobs := []orchestrator.JobInfo{
				{ID: "JOB-1", Status: "Running"},
			}
			json.NewEncoder(w).Encode(jobs)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Execute Cmd
	cmd := fetchStatus(server.URL)
	msg := cmd()

	// Verify
	statusMsg, ok := msg.(statusMsg)
	require.True(t, ok)
	assert.NoError(t, statusMsg.Err)
	assert.Equal(t, "1h", statusMsg.Status.Uptime)
	assert.Len(t, statusMsg.Jobs, 1)
	assert.Equal(t, "JOB-1", statusMsg.Jobs[0].ID)
}

func TestFetchStatus_Error(t *testing.T) {
	// Closed server to force connection error
	server := httptest.NewServer(http.HandlerFunc(nil))
	server.Close()

	cmd := fetchStatus(server.URL)
	msg := cmd()

	statusMsg, ok := msg.(statusMsg)
	require.True(t, ok)
	assert.Error(t, statusMsg.Err)
}

func TestFetchStatus_JSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	cmd := fetchStatus(server.URL)
	msg := cmd()

	statusMsg, ok := msg.(statusMsg)
	require.True(t, ok)
	assert.Error(t, statusMsg.Err)
}

func TestInit(t *testing.T) {
	model := DashboardModel{}
	cmd := model.Init()
	assert.NotNil(t, cmd)
}

func TestTick(t *testing.T) {
	cmd := tick()
	assert.NotNil(t, cmd)
}
