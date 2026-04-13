package tui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApproveJobCmd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/test-id/approve", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cmd := approveJobCmd(ts.URL, "test-id")
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actMsg.Err)
	assert.Equal(t, "Approved", actMsg.Message)
}

func TestApproveJobCmd_Error(t *testing.T) {
	cmd := approveJobCmd("http://localhost:0", "test-id") // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestApproveJobCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer ts.Close()

	cmd := approveJobCmd(ts.URL, "test-id")
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
	assert.Contains(t, actMsg.Err.Error(), "server error")
}

func TestApproveJobCmd_InvalidURL(t *testing.T) {
	cmd := approveJobCmd("::invalid-url", "test-id") // Causes NewRequest to fail
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}
