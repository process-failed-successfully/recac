package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHoldJobs(t *testing.T) {
	tests := []struct {
		name           string
		match          string
		tag            string
		handler        http.HandlerFunc
		expectedOutput string
		expectedExit   int
	}{
		{
			name:  "success",
			match: "test-match",
			tag:   "test-tag",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/jobs/hold", r.URL.Path)
				assert.Equal(t, "test-match", r.URL.Query().Get("match"))
				assert.Equal(t, "test-tag", r.URL.Query().Get("tag"))

				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]int{"held": 5})
			},
			expectedOutput: "Successfully held 5 jobs.\n",
			expectedExit:   0,
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("internal error"))
			},
			expectedOutput: "Failed to hold jobs: internal error\n",
			expectedExit:   1,
		},
		{
			name: "invalid url",
			handler: nil,
			expectedOutput: "Failed to parse URL",
			expectedExit:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var serverURL string
			if tt.name == "invalid url" {
				serverURL = "://invalid"
			} else if tt.handler != nil {
				ts := httptest.NewServer(tt.handler)
				defer ts.Close()
				serverURL = ts.URL
			} else {
				serverURL = "http://127.0.0.1:0" // Force connection error
			}

			var buf bytes.Buffer
			origStdout := stdout
			stdout = &buf
			defer func() { stdout = origStdout }()

			var exitCode int
			origExitFunc := exitFunc
			exitFunc = func(code int) {
				exitCode = code
			}
			defer func() { exitFunc = origExitFunc }()

			holdJobs(serverURL, tt.match, tt.tag)

			if tt.expectedExit != exitCode {
				t.Errorf("expected exit code %d, got %d", tt.expectedExit, exitCode)
			}
			if !bytes.Contains(buf.Bytes(), []byte(tt.expectedOutput)) && string(buf.Bytes()) != tt.expectedOutput {
				t.Errorf("expected output containing %q, got %q", tt.expectedOutput, buf.String())
			}
		})
	}
}

func TestUnholdJobs(t *testing.T) {
	tests := []struct {
		name           string
		match          string
		tag            string
		handler        http.HandlerFunc
		expectedOutput string
		expectedExit   int
	}{
		{
			name:  "success",
			match: "test-match",
			tag:   "test-tag",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/jobs/unhold", r.URL.Path)
				assert.Equal(t, "test-match", r.URL.Query().Get("match"))
				assert.Equal(t, "test-tag", r.URL.Query().Get("tag"))

				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]int{"unheld": 3})
			},
			expectedOutput: "Successfully unheld 3 jobs.\n",
			expectedExit:   0,
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("internal error"))
			},
			expectedOutput: "Failed to unhold jobs: internal error\n",
			expectedExit:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var serverURL string
			if tt.handler != nil {
				ts := httptest.NewServer(tt.handler)
				defer ts.Close()
				serverURL = ts.URL
			}

			var buf bytes.Buffer
			origStdout := stdout
			stdout = &buf
			defer func() { stdout = origStdout }()

			var exitCode int
			origExitFunc := exitFunc
			exitFunc = func(code int) {
				exitCode = code
			}
			defer func() { exitFunc = origExitFunc }()

			unholdJobs(serverURL, tt.match, tt.tag)

			if tt.expectedExit != exitCode {
				t.Errorf("expected exit code %d, got %d", tt.expectedExit, exitCode)
			}
			if !bytes.Contains(buf.Bytes(), []byte(tt.expectedOutput)) && string(buf.Bytes()) != tt.expectedOutput {
				t.Errorf("expected output containing %q, got %q", tt.expectedOutput, buf.String())
			}
		})
	}
}

func TestHoldJob(t *testing.T) {
	tests := []struct {
		name           string
		jobID          string
		handler        http.HandlerFunc
		expectedOutput string
		expectedExit   int
	}{
		{
			name:  "success",
			jobID: "job-1",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/jobs/job-1/hold", r.URL.Path)
				w.WriteHeader(http.StatusOK)
			},
			expectedOutput: "Job job-1 held successfully.\n",
			expectedExit:   0,
		},
		{
			name:  "server error",
			jobID: "job-1",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("internal error"))
			},
			expectedOutput: "Failed to hold job: internal error\n",
			expectedExit:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var serverURL string
			if tt.handler != nil {
				ts := httptest.NewServer(tt.handler)
				defer ts.Close()
				serverURL = ts.URL
			}

			var buf bytes.Buffer
			origStdout := stdout
			stdout = &buf
			defer func() { stdout = origStdout }()

			var exitCode int
			origExitFunc := exitFunc
			exitFunc = func(code int) {
				exitCode = code
			}
			defer func() { exitFunc = origExitFunc }()

			holdJob(serverURL, tt.jobID)

			if tt.expectedExit != exitCode {
				t.Errorf("expected exit code %d, got %d", tt.expectedExit, exitCode)
			}
			if !bytes.Contains(buf.Bytes(), []byte(tt.expectedOutput)) && string(buf.Bytes()) != tt.expectedOutput {
				t.Errorf("expected output containing %q, got %q", tt.expectedOutput, buf.String())
			}
		})
	}
}

func TestUnholdJob(t *testing.T) {
	tests := []struct {
		name           string
		jobID          string
		handler        http.HandlerFunc
		expectedOutput string
		expectedExit   int
	}{
		{
			name:  "success",
			jobID: "job-1",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/jobs/job-1/unhold", r.URL.Path)
				w.WriteHeader(http.StatusOK)
			},
			expectedOutput: "Job job-1 unheld successfully.\n",
			expectedExit:   0,
		},
		{
			name:  "server error",
			jobID: "job-1",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("internal error"))
			},
			expectedOutput: "Failed to unhold job: internal error\n",
			expectedExit:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var serverURL string
			if tt.handler != nil {
				ts := httptest.NewServer(tt.handler)
				defer ts.Close()
				serverURL = ts.URL
			}

			var buf bytes.Buffer
			origStdout := stdout
			stdout = &buf
			defer func() { stdout = origStdout }()

			var exitCode int
			origExitFunc := exitFunc
			exitFunc = func(code int) {
				exitCode = code
			}
			defer func() { exitFunc = origExitFunc }()

			unholdJob(serverURL, tt.jobID)

			if tt.expectedExit != exitCode {
				t.Errorf("expected exit code %d, got %d", tt.expectedExit, exitCode)
			}
			if !bytes.Contains(buf.Bytes(), []byte(tt.expectedOutput)) && string(buf.Bytes()) != tt.expectedOutput {
				t.Errorf("expected output containing %q, got %q", tt.expectedOutput, buf.String())
			}
		})
	}
}
