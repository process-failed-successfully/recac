package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestCleanAll_Success(t *testing.T) {
	var cancelCalled, pendingCalled, historyCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs" && r.Method == http.MethodDelete {
			cancelCalled = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"canceled": 5}`))
			return
		}
		if r.URL.Path == "/pending" && r.Method == http.MethodDelete {
			pendingCalled = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"cleared": 3}`))
			return
		}
		if r.URL.Path == "/history" && r.Method == http.MethodDelete {
			historyCalled = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"cleared": 10}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	viper.Set("orchestrator.host", server.URL)
	viper.Set("orchestrator.clean_all", true)
	defer viper.Reset()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = oldExit }()

	err := run(nil, nil)

	assert.NoError(t, err)
	assert.False(t, exitCalled)
	assert.True(t, cancelCalled, "cancelAllJobs should be called")
	assert.True(t, pendingCalled, "clearPending should be called")
	assert.True(t, historyCalled, "clearHistory should be called")

	output := buf.String()
	assert.Contains(t, output, "Successfully canceled 5 jobs.")
	assert.Contains(t, output, "Successfully cleared 3 jobs from pending queue.")
	assert.Contains(t, output, "Successfully cleared 10 jobs from history.")
}
