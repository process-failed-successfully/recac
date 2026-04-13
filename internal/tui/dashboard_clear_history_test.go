package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClearHistoryCmd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/history", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)

		result := map[string]interface{}{"cleared": 15.0}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer ts.Close()

	cmd := clearHistory(ts.URL)
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actMsg.Err)
	assert.Equal(t, "Cleared 15 jobs", actMsg.Message)
}

func TestClearHistoryCmd_Error(t *testing.T) {
	cmd := clearHistory("http://localhost:0") // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestClearHistoryCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cmd := clearHistory(ts.URL)
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
}

func TestClearHistoryCmd_InvalidURL(t *testing.T) {
	cmd := clearHistory("::invalid-url") // Causes NewRequest to fail
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestClearHistoryCmd_DecodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer ts.Close()

	cmd := clearHistory(ts.URL)
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "failed to parse response")
}

func TestClearHistoryCmd_InvalidFormatError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{"cleared": "not-a-number"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer ts.Close()

	cmd := clearHistory(ts.URL)
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "invalid response format")
}
