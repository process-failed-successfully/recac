package tui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHoldJobCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/jobs/job-123/hold", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cmd := holdJobCmd(server.URL, "job-123")
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actionMsg.Err)
	assert.Equal(t, "Held", actionMsg.Message)
}

func TestHoldJobCmd_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/jobs/job-123/hold", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cmd := holdJobCmd(server.URL, "job-123")
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actionMsg.Err)
	assert.Contains(t, actionMsg.Err.Error(), "status 500")
}

func TestUnholdJobCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/jobs/job-123/unhold", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cmd := unholdJobCmd(server.URL, "job-123")
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actionMsg.Err)
	assert.Equal(t, "Unheld", actionMsg.Message)
}

func TestUnholdJobCmd_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/jobs/job-123/unhold", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cmd := unholdJobCmd(server.URL, "job-123")
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actionMsg.Err)
	assert.Contains(t, actionMsg.Err.Error(), "status 500")
}
