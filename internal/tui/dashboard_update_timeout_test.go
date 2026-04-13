package tui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateTimeoutCmd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/test-id/timeout", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.JSONEq(t, `{"timeout": "10m"}`, string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cmd := updateTimeoutCmd(ts.URL, "test-id", "10m")
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actMsg.Err)
	assert.Equal(t, "Updated timeout for job test-id to 10m", actMsg.Message)
}

func TestUpdateTimeoutCmd_Error(t *testing.T) {
	cmd := updateTimeoutCmd("http://localhost:0", "test-id", "10m") // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestUpdateTimeoutCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer ts.Close()

	cmd := updateTimeoutCmd(ts.URL, "test-id", "10m")
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
	assert.Contains(t, actMsg.Err.Error(), "server error")
}

func TestUpdateTimeoutCmd_InvalidURL(t *testing.T) {
	cmd := updateTimeoutCmd("::invalid-url", "test-id", "10m") // Causes NewRequest to fail
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}
