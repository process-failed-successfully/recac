package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
)

func TestFetchSimulateCmd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/simulate", r.URL.Path)

		report := orchestrator.SimulationReport{
			EstimatedTotalCost: 12.34,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(report)
	}))
	defer ts.Close()

	cmd := fetchSimulateCmd(ts.URL)
	msg := cmd()

	simMsg, ok := msg.(simulateMsg)
	assert.True(t, ok)
	assert.NoError(t, simMsg.Err)
	assert.Equal(t, 12.34, simMsg.Report.EstimatedTotalCost)
}

func TestFetchSimulateCmd_Error(t *testing.T) {
	cmd := fetchSimulateCmd("http://localhost:0") // Invalid port
	msg := cmd()

	simMsg, ok := msg.(simulateMsg)
	assert.True(t, ok)
	assert.Error(t, simMsg.Err)
}

func TestFetchSimulateCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cmd := fetchSimulateCmd(ts.URL)
	msg := cmd()

	simMsg, ok := msg.(simulateMsg)
	assert.True(t, ok)
	assert.Error(t, simMsg.Err)
	assert.Contains(t, simMsg.Err.Error(), "HTTP 500")
}

func TestFetchSimulateCmd_DecodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer ts.Close()

	cmd := fetchSimulateCmd(ts.URL)
	msg := cmd()

	simMsg, ok := msg.(simulateMsg)
	assert.True(t, ok)
	assert.Error(t, simMsg.Err)
}
