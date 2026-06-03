package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
)

func TestCancelInteractive(t *testing.T) {
	tests := []struct {
		name            string
		activeJobs      []orchestrator.JobInfo
		pendingJobs     []orchestrator.JobInfo
		inputStr        string
		expectedOutput  []string
		expectedExit    int
		expectedMethods map[string]string // URL Path -> HTTP Method expected
	}{
		{
			name:        "No active or pending jobs",
			activeJobs:  []orchestrator.JobInfo{},
			pendingJobs: []orchestrator.JobInfo{},
			inputStr:    "",
			expectedOutput: []string{
				"No active or pending jobs are currently cancellable.",
			},
			expectedExit:    0,
			expectedMethods: map[string]string{},
		},
		{
			name: "Skip job",
			activeJobs: []orchestrator.JobInfo{
				{ID: "JOB-1", Status: "Running", Summary: "Job 1"},
			},
			pendingJobs: []orchestrator.JobInfo{},
			inputStr:    "s\n",
			expectedOutput: []string{
				"Interactive Cancel (1 jobs)",
				"ID:", "JOB-1",
				"Summary:", "Job 1",
				"Skipping JOB-1.",
				"All cancellable jobs processed.",
			},
			expectedExit:    0,
			expectedMethods: map[string]string{},
		},
		{
			name: "Cancel job",
			activeJobs: []orchestrator.JobInfo{
				{ID: "JOB-1", Status: "Running", Summary: "Job 1"},
			},
			pendingJobs: []orchestrator.JobInfo{},
			inputStr:    "c\n",
			expectedOutput: []string{
				"Interactive Cancel (1 jobs)",
				"ID:", "JOB-1",
				"Job JOB-1 cancelled successfully.",
				"All cancellable jobs processed.",
			},
			expectedExit: 0,
			expectedMethods: map[string]string{
				"/jobs/JOB-1": http.MethodDelete,
			},
		},
		{
			name:       "Quit midway",
			activeJobs: []orchestrator.JobInfo{
				{ID: "JOB-1", Status: "Running", Summary: "Job 1"},
				{ID: "JOB-2", Status: "Pending", Summary: "Job 2"},
			},
			pendingJobs: []orchestrator.JobInfo{},
			inputStr:    "q\n",
			expectedOutput: []string{
				"Interactive Cancel (2 jobs)",
				"ID:", "JOB-1",
				"Exiting interactive cancel.",
			},
			expectedExit: 0,
			expectedMethods: map[string]string{},
		},
		{
			name:       "Invalid input then cancel",
			activeJobs: []orchestrator.JobInfo{
				{ID: "JOB-1", Status: "Running", Summary: "Job 1"},
			},
			pendingJobs: []orchestrator.JobInfo{},
			inputStr:    "x\nc\n",
			expectedOutput: []string{
				"Invalid input. Please enter 'c', 's', or 'q'.",
				"Job JOB-1 cancelled successfully.",
			},
			expectedExit: 0,
			expectedMethods: map[string]string{
				"/jobs/JOB-1": http.MethodDelete,
			},
		},
		{
			name:       "Multiple jobs cancel and skip",
			activeJobs: []orchestrator.JobInfo{
				{ID: "JOB-1", Status: "Running", Summary: "Job 1"},
			},
			pendingJobs: []orchestrator.JobInfo{
				{ID: "JOB-2", Status: "Pending", Summary: "Job 2"},
			},
			inputStr:    "c\ns\n",
			expectedOutput: []string{
				"Interactive Cancel (2 jobs)",
				"Job JOB-1 cancelled successfully.",
				"Skipping JOB-2.",
				"All cancellable jobs processed.",
			},
			expectedExit: 0,
			expectedMethods: map[string]string{
				"/jobs/JOB-1": http.MethodDelete,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			methodsCalled := make(map[string]string)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				methodsCalled[r.URL.Path] = r.Method

				if r.URL.Path == "/jobs" {
					w.WriteHeader(http.StatusOK)
					if r.URL.Query().Get("state") == "active" {
						json.NewEncoder(w).Encode(tt.activeJobs)
					} else if r.URL.Query().Get("state") == "pending" {
						json.NewEncoder(w).Encode(tt.pendingJobs)
					} else {
						json.NewEncoder(w).Encode([]orchestrator.JobInfo{})
					}
					return
				}

				if r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/jobs/") {
					w.WriteHeader(http.StatusOK)
					return
				}

				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			oldStdout := stdout
			oldStdin := stdin
			oldExit := exitFunc

			rStdin, wStdin, _ := os.Pipe()
			wStdin.Write([]byte(tt.inputStr))
			wStdin.Close()
			stdin = rStdin

			var buf bytes.Buffer
			stdout = &buf

			exitCode := 0
			exitFunc = func(code int) {
				exitCode = code
				panic("exit")
			}

			defer func() {
				stdout = oldStdout
				stdin = oldStdin
				exitFunc = oldExit

				if r := recover(); r != nil {
					if r != "exit" {
						panic(r)
					}
				}

				output := buf.String()
				for _, expected := range tt.expectedOutput {
					assert.Contains(t, output, expected, "Output missing expected string")
				}
				assert.Equal(t, tt.expectedExit, exitCode, "Exit code mismatch")

				for path, method := range tt.expectedMethods {
					actualMethod, ok := methodsCalled[path]
					assert.True(t, ok, "Expected path not called: %s", path)
					assert.Equal(t, method, actualMethod, "HTTP method mismatch for %s", path)
				}
			}()

			cancelInteractive(server.URL)
		})
	}
}

func TestCancelInteractive_ParseError(t *testing.T) {
	oldStdout := stdout
	oldExit := exitFunc

	var buf bytes.Buffer
	stdout = &buf

	exitCode := 0
	exitFunc = func(code int) {
		exitCode = code
		panic("exit")
	}

	defer func() {
		stdout = oldStdout
		exitFunc = oldExit
		recover()
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to parse host URL")
	}()

	cancelInteractive("://invalid-url")
}

func TestCancelInteractive_ConnectionError(t *testing.T) {
	oldStdout := stdout
	oldExit := exitFunc

	var buf bytes.Buffer
	stdout = &buf

	exitCode := 0
	exitFunc = func(code int) {
		exitCode = code
		panic("exit")
	}

	defer func() {
		stdout = oldStdout
		exitFunc = oldExit
		recover()
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	}()

	cancelInteractive("http://localhost:0")
}

func TestCancelInteractive_StatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	oldStdout := stdout
	oldExit := exitFunc

	var buf bytes.Buffer
	stdout = &buf

	exitCode := 0
	exitFunc = func(code int) {
		exitCode = code
		panic("exit")
	}

	defer func() {
		stdout = oldStdout
		exitFunc = oldExit
		recover()
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to fetch active jobs: status 500")
	}()

	cancelInteractive(server.URL)
}

func TestCancelInteractive_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid-json`))
	}))
	defer server.Close()

	oldStdout := stdout
	oldExit := exitFunc

	var buf bytes.Buffer
	stdout = &buf

	exitCode := 0
	exitFunc = func(code int) {
		exitCode = code
		panic("exit")
	}

	defer func() {
		stdout = oldStdout
		exitFunc = oldExit
		recover()
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to decode response")
	}()

	cancelInteractive(server.URL)
}
