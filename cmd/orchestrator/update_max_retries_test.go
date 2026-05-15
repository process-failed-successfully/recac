package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateMaxRetries_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/TEST-123/max-retries", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)

		var reqBody struct {
			MaxRetries int `json:"max_retries"`
		}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		assert.NoError(t, err)
		assert.Equal(t, 5, reqBody.MaxRetries)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"max_retries": 5}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	updateMaxRetries(server.URL, "TEST-123", 5)

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Job TEST-123 max retries updated successfully to 5.")
}

func TestUpdateMaxRetries_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/TEST-123/max-retries", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`job not found`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateMaxRetries(server.URL, "TEST-123", 5)

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to update max retries: job not found")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkMaxRetries_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/max-retries", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)

		tag := r.URL.Query().Get("tag")
		assert.Equal(t, "backend", tag)

		var reqBody struct {
			MaxRetries int `json:"max_retries"`
		}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		assert.NoError(t, err)
		assert.Equal(t, 10, reqBody.MaxRetries)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"updated": 3}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	updateBulkMaxRetries(server.URL, "", "backend", 10)

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Successfully updated max retries for 3 pending jobs.")
}

func TestUpdateMaxRetries_ConnectionError(t *testing.T) {
	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateMaxRetries("http://localhost:0", "TEST-123", 5)

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkMaxRetries_InvalidURL(t *testing.T) {
	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkMaxRetries(":\x7f//invalid", "match-me", "", 10)

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to parse URL")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkMaxRetries_JSONError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/max-retries", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkMaxRetries(server.URL, "match-me", "", 10)

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to decode response")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateMaxRetries_InvalidRequest(t *testing.T) {
	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateMaxRetries("http://localhost\x7f", "TEST-123", 5)

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to create request")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkMaxRetries_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/max-retries", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`internal server error`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkMaxRetries(server.URL, "match-me", "", 10)

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to update bulk max retries: internal server error")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkMaxRetries_ConnectionError(t *testing.T) {
	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkMaxRetries("http://localhost:0", "match-me", "", 10)

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}
