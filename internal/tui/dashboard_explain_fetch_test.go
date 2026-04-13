package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFetchExplanationCmd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/test-id/explain", r.URL.Path)

		explanation := struct {
			Explanation string `json:"explanation"`
		}{
			Explanation: "This is a test explanation",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(explanation)
	}))
	defer ts.Close()

	cmd := fetchExplanation(ts.URL, "test-id")
	msg := cmd()

	expMsg, ok := msg.(explainMsg)
	assert.True(t, ok)
	assert.NoError(t, expMsg.Err)
	assert.Equal(t, "This is a test explanation", expMsg.Explanation)
}

func TestFetchExplanationCmd_Error(t *testing.T) {
	cmd := fetchExplanation("http://localhost:0", "test-id") // Invalid port
	msg := cmd()

	expMsg, ok := msg.(explainMsg)
	assert.True(t, ok)
	assert.Error(t, expMsg.Err)
}

func TestFetchExplanationCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer ts.Close()

	cmd := fetchExplanation(ts.URL, "test-id")
	msg := cmd()

	expMsg, ok := msg.(explainMsg)
	assert.True(t, ok)
	assert.Error(t, expMsg.Err)
	assert.Contains(t, expMsg.Err.Error(), "status 500")
	assert.Contains(t, expMsg.Err.Error(), "server error")
}

func TestFetchExplanationCmd_DecodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer ts.Close()

	cmd := fetchExplanation(ts.URL, "test-id")
	msg := cmd()

	expMsg, ok := msg.(explainMsg)
	assert.True(t, ok)
	assert.Error(t, expMsg.Err)
}
