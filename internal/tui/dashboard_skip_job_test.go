package tui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestSkipJobCmd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/JOB-1/skip" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cmd := skipJobCmd(ts.URL, "JOB-1")
	msg := cmd()
	aMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Nil(t, aMsg.Err)
	assert.Equal(t, "Skipped", aMsg.Message)

	// Test error status
	cmd = skipJobCmd(ts.URL, "UNKNOWN")
	msg = cmd()
	aMsg, ok = msg.(actionMsg)
	assert.True(t, ok)
	assert.NotNil(t, aMsg.Err)
	assert.Contains(t, aMsg.Err.Error(), "status 404")

    // Test bad url
    cmd = skipJobCmd("::badurl", "JOB-1")
    msg = cmd()
    aMsg, ok = msg.(actionMsg)
	assert.True(t, ok)
	assert.NotNil(t, aMsg.Err)

    // Test no server
    cmd = skipJobCmd("http://127.0.0.1:0", "JOB-1")
    msg = cmd()
    aMsg, ok = msg.(actionMsg)
	assert.True(t, ok)
	assert.NotNil(t, aMsg.Err)
}

func TestSkipJobDownstreamCmd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/JOB-1/skip" && r.URL.Query().Get("downstream") == "true" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cmd := skipJobDownstreamCmd(ts.URL, "JOB-1")
	msg := cmd()
	aMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Nil(t, aMsg.Err)
	assert.Equal(t, "Skipped (Downstream)", aMsg.Message)

	// Test error status
	cmd = skipJobDownstreamCmd(ts.URL, "UNKNOWN")
	msg = cmd()
	aMsg, ok = msg.(actionMsg)
	assert.True(t, ok)
	assert.NotNil(t, aMsg.Err)
	assert.Contains(t, aMsg.Err.Error(), "status 404")

    // Test bad url
    cmd = skipJobDownstreamCmd("::badurl", "JOB-1")
    msg = cmd()
    aMsg, ok = msg.(actionMsg)
	assert.True(t, ok)
	assert.NotNil(t, aMsg.Err)

    // Test no server
    cmd = skipJobDownstreamCmd("http://127.0.0.1:0", "JOB-1")
    msg = cmd()
    aMsg, ok = msg.(actionMsg)
	assert.True(t, ok)
	assert.NotNil(t, aMsg.Err)
}
