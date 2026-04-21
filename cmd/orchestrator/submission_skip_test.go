package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSkipJobs(t *testing.T) {
	tests := []struct {
		name           string
		match          string
		tag            string
		group          string
		handler        http.HandlerFunc
		expectedOutput string
		expectedExit   int
	}{
		{
			name:  "success",
			match: "test-match",
			tag:   "test-tag",
			group: "test-group",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/jobs/skip", r.URL.Path)
				assert.Equal(t, "test-match", r.URL.Query().Get("match"))
				assert.Equal(t, "test-tag", r.URL.Query().Get("tag"))
				assert.Equal(t, "test-group", r.URL.Query().Get("group"))

				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]int{"skipped": 5})
			},
			expectedOutput: "Successfully skipped 5 jobs.\n",
			expectedExit:   0,
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("internal error"))
			},
			expectedOutput: "Failed to skip jobs: internal error\n",
			expectedExit:   1,
		},
		{
			name:           "invalid url",
			handler:        nil,
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

			skipJobs(serverURL, tt.match, tt.tag, tt.group)

			if tt.expectedExit != exitCode {
				t.Errorf("expected exit code %d, got %d", tt.expectedExit, exitCode)
			}
			if !bytes.Contains(buf.Bytes(), []byte(tt.expectedOutput)) && string(buf.Bytes()) != tt.expectedOutput {
				t.Errorf("expected output containing %q, got %q", tt.expectedOutput, buf.String())
			}
		})
	}
}

func TestSkipJob(t *testing.T) {
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
				assert.Equal(t, "/jobs/job-1/skip", r.URL.Path)
				w.WriteHeader(http.StatusOK)
			},
			expectedOutput: "Job job-1 skipped successfully.\n",
			expectedExit:   0,
		},
		{
			name:  "server error",
			jobID: "job-1",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("internal error"))
			},
			expectedOutput: "Failed to skip job: internal error\n",
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

			skipJob(serverURL, tt.jobID)

			if tt.expectedExit != exitCode {
				t.Errorf("expected exit code %d, got %d", tt.expectedExit, exitCode)
			}
			if !bytes.Contains(buf.Bytes(), []byte(tt.expectedOutput)) && string(buf.Bytes()) != tt.expectedOutput {
				t.Errorf("expected output containing %q, got %q", tt.expectedOutput, buf.String())
			}
		})
	}
}
