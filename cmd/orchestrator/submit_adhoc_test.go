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

func TestSubmitAdHocJob_SuccessNoWait(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Job accepted"))
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

	submitAdHocJob(server.URL, "repo", "task", "id", 0, 0, 0, false, nil, nil, nil)

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "Job accepted")
	assert.Equal(t, 0, exitCode)
}

func TestSubmitAdHocJob_SuccessWait(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs" {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("Job accepted"))
			return
		}
		if r.URL.Path == "/jobs/id" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"Status": "Completed"}`))
			return
		}
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

	submitAdHocJob(server.URL, "repo", "task", "id", 0, 0, 0, true, nil, nil, nil)

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "Job accepted")
	assert.Contains(t, buf.String(), "Job already completed")
	assert.Equal(t, 0, exitCode)
}

func TestSubmitAdHocJob_WaitFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs" {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("Job accepted"))
			return
		}
		if r.URL.Path == "/jobs/id" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"Status": "Failed", "Error": "something bad"}`))
			return
		}
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

	submitAdHocJob(server.URL, "repo", "task", "id", 0, 0, 0, true, nil, nil, nil)

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "Job failed: job failed with error: something bad")
	assert.Equal(t, 1, exitCode)
}
