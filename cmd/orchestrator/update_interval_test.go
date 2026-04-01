package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestUpdatePollIntervalCmd(t *testing.T) {
	var capturedBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/interval", r.URL.Path)

		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		capturedBody = buf.String()

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	viper.Set("orchestrator.host", ts.URL)
	defer viper.Reset()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Normal run
	updatePollInterval(ts.URL, "2m")

	assert.Contains(t, buf.String(), "Orchestrator poll interval updated to 2m")
	assert.Contains(t, capturedBody, `{"interval": "2m"}`)
}

func TestUpdatePollIntervalCmd_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "invalid interval format")
	}))
	defer ts.Close()

	viper.Set("orchestrator.host", ts.URL)
	defer viper.Reset()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	defer func() { exitFunc = oldExit }()

	exitFunc = func(code int) {
		panic(fmt.Sprintf("exit %d", code))
	}

	assert.PanicsWithValue(t, "exit 1", func() {
		updatePollInterval(ts.URL, "invalid")
	})

	assert.Contains(t, buf.String(), "Failed to update poll interval")
}
