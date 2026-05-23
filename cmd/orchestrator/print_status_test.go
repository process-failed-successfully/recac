package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintStatus_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/status", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"uptime": "10h", "active_spawns": 2, "draining": true}`))
	}))
	defer server.Close()

	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	printStatus(server.URL, "text")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "Orchestrator Status")
	assert.Contains(t, buf.String(), "10h")
	assert.Contains(t, buf.String(), "Draining:")
	assert.Contains(t, buf.String(), "true")
	assert.Equal(t, 0, exitCode)
}

func TestPrintStatus_ConnectionError(t *testing.T) {
	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	printStatus("http://invalid-url-that-will-fail.test", "text")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestPrintStatus_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	printStatus(server.URL, "text")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "Failed to fetch status: status 500 Internal Server Error")
	assert.Equal(t, 1, exitCode)
}

func TestPrintStatus_FormatJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/status", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"uptime": "15h", "active_spawns": 4, "draining": true}`))
	}))
	defer server.Close()

	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	printStatus(server.URL, "json")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, `"uptime": "15h"`)
	assert.Contains(t, output, `"active_spawns": 4`)
	assert.Contains(t, output, `"draining": true`)
	assert.NotContains(t, output, "Orchestrator Status") // Human readable text shouldn't be here
	assert.Equal(t, 0, exitCode)
}

func TestPrintStatus_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	oldExit := exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	printStatus(server.URL, "text")

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "Failed to decode response")
	assert.Equal(t, 1, exitCode)
}
