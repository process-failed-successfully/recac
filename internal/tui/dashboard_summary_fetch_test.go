package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFetchSummaryCmd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/summary", r.URL.Path)

		summary := map[string]int{"completed": 5, "pending": 2}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summary)
	}))
	defer ts.Close()

	cmd := fetchSummaryCmd(ts.URL)
	msg := cmd()

	sumMsg, ok := msg.(summaryMsg)
	assert.True(t, ok)
	assert.NoError(t, sumMsg.Err)
	assert.Equal(t, 5, sumMsg.Summary["completed"])
	assert.Equal(t, 2, sumMsg.Summary["pending"])
}

func TestFetchSummaryCmd_Error(t *testing.T) {
	cmd := fetchSummaryCmd("http://localhost:0") // Invalid port
	msg := cmd()

	sumMsg, ok := msg.(summaryMsg)
	assert.True(t, ok)
	assert.Error(t, sumMsg.Err)
}

func TestFetchSummaryCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer ts.Close()

	cmd := fetchSummaryCmd(ts.URL)
	msg := cmd()

	sumMsg, ok := msg.(summaryMsg)
	assert.True(t, ok)
	assert.Error(t, sumMsg.Err)
	assert.Contains(t, sumMsg.Err.Error(), "status 500")
	assert.Contains(t, sumMsg.Err.Error(), "server error")
}

func TestFetchSummaryCmd_DecodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer ts.Close()

	cmd := fetchSummaryCmd(ts.URL)
	msg := cmd()

	sumMsg, ok := msg.(summaryMsg)
	assert.True(t, ok)
	assert.Error(t, sumMsg.Err)
}
