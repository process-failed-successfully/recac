package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFetchAnalyzeReliabilityCmd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/analyze/reliability", r.URL.Path)
		assert.Equal(t, "10", r.URL.Query().Get("limit"))

		report := ReliabilityStats{
			TotalJobs: 10,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(report)
	}))
	defer ts.Close()

	cmd := fetchAnalyzeReliabilityCmd(ts.URL)
	msg := cmd()

	actMsg, ok := msg.(analyzeReliabilityMsg)
	assert.True(t, ok)
	assert.NoError(t, actMsg.Err)
	assert.Equal(t, 10, actMsg.Stats.TotalJobs)
}

func TestFetchAnalyzeReliabilityCmd_Error(t *testing.T) {
	cmd := fetchAnalyzeReliabilityCmd("http://localhost:0") // Invalid port
	msg := cmd()

	actMsg, ok := msg.(analyzeReliabilityMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestFetchAnalyzeReliabilityCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer ts.Close()

	cmd := fetchAnalyzeReliabilityCmd(ts.URL)
	msg := cmd()

	actMsg, ok := msg.(analyzeReliabilityMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
	assert.Contains(t, actMsg.Err.Error(), "server error")
}

func TestFetchAnalyzeReliabilityCmd_InvalidURL(t *testing.T) {
	cmd := fetchAnalyzeReliabilityCmd("::invalid-url") // Parse error
	msg := cmd()

	actMsg, ok := msg.(analyzeReliabilityMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestFetchAnalyzeReliabilityCmd_DecodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer ts.Close()

	cmd := fetchAnalyzeReliabilityCmd(ts.URL)
	msg := cmd()

	actMsg, ok := msg.(analyzeReliabilityMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}
