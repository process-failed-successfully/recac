package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFetchAnalyzeDurationsCmd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/analyze/durations", r.URL.Path)
		assert.Equal(t, "10", r.URL.Query().Get("limit"))

		report := DurationStats{
			TotalJobs: 10,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(report)
	}))
	defer ts.Close()

	cmd := fetchAnalyzeDurationsCmd(ts.URL)
	msg := cmd()

	actMsg, ok := msg.(analyzeDurationsMsg)
	assert.True(t, ok)
	assert.NoError(t, actMsg.Err)
	assert.Equal(t, 10, actMsg.Stats.TotalJobs)
}

func TestFetchAnalyzeDurationsCmd_Error(t *testing.T) {
	cmd := fetchAnalyzeDurationsCmd("http://localhost:0") // Invalid port
	msg := cmd()

	actMsg, ok := msg.(analyzeDurationsMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestFetchAnalyzeDurationsCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer ts.Close()

	cmd := fetchAnalyzeDurationsCmd(ts.URL)
	msg := cmd()

	actMsg, ok := msg.(analyzeDurationsMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
	assert.Contains(t, actMsg.Err.Error(), "server error")
}

func TestFetchAnalyzeDurationsCmd_InvalidURL(t *testing.T) {
	cmd := fetchAnalyzeDurationsCmd("::invalid-url") // Parse error
	msg := cmd()

	actMsg, ok := msg.(analyzeDurationsMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestFetchAnalyzeDurationsCmd_DecodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer ts.Close()

	cmd := fetchAnalyzeDurationsCmd(ts.URL)
	msg := cmd()

	actMsg, ok := msg.(analyzeDurationsMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}
