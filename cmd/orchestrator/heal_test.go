package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"recac/internal/orchestrator"
)

func TestHealBulkJobs(t *testing.T) {
	tests := []struct {
		name         string
		match        string
		tag          string
		statusCode   int
		response     string
		expectedExit int
		expectOutput []string
	}{
		{
			name:         "Success Match",
			match:        "failed-job",
			tag:          "",
			statusCode:   http.StatusOK,
			response:     `{"healed": 3}`,
			expectedExit: -1,
			expectOutput: []string{
				"Successfully healed 3 failed jobs.",
			},
		},
		{
			name:         "Error connection failed",
			match:        "failed-job",
			tag:          "",
			statusCode:   -1, // Simulate connection error
			response:     ``,
			expectedExit: 1,
			expectOutput: []string{
				"Failed to connect to orchestrator",
			},
		},
		{
			name:         "Error invalid URL",
			match:        "failed-job",
			tag:          "",
			statusCode:   -2, // Simulate invalid url
			response:     ``,
			expectedExit: 1,
			expectOutput: []string{
				"Failed to parse URL:",
			},
		},
		{
			name:         "Success Tag",
			match:        "",
			tag:          "bug",
			statusCode:   http.StatusOK,
			response:     `{"healed": 5}`,
			expectedExit: -1,
			expectOutput: []string{
				"Successfully healed 5 failed jobs.",
			},
		},
		{
			name:         "API Error",
			match:        "test",
			tag:          "",
			statusCode:   http.StatusInternalServerError,
			response:     `internal error`,
			expectedExit: 1,
			expectOutput: []string{
				"Failed to heal jobs: internal error",
			},
		},
		{
			name:         "Invalid JSON",
			match:        "test",
			tag:          "",
			statusCode:   http.StatusOK,
			response:     `{invalid json}`,
			expectedExit: 1,
			expectOutput: []string{
				"Failed to decode response: invalid character",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/jobs/heal/bulk", r.URL.Path)

				if tc.match != "" {
					assert.Equal(t, tc.match, r.URL.Query().Get("match"))
				}
				if tc.tag != "" {
					assert.Equal(t, tc.tag, r.URL.Query().Get("tag"))
				}

				w.WriteHeader(tc.statusCode)
				w.Write([]byte(tc.response))
			}))
			defer server.Close()

			var buf bytes.Buffer
			originalStdout := stdout
			stdout = &buf
			defer func() { stdout = originalStdout }()

			originalExitFunc := exitFunc
			exitCode := -1
			exitFunc = func(code int) {
				exitCode = code
			}
			defer func() { exitFunc = originalExitFunc }()

			if tc.statusCode == -2 {
				healBulkJobs("http://[::1]:namedport", tc.match, tc.tag)
			} else if tc.statusCode == -1 {
				server.Close() // Force connection error
				healBulkJobs(server.URL, tc.match, tc.tag)
			} else {
				healBulkJobs(server.URL, tc.match, tc.tag)
			}

			assert.Equal(t, tc.expectedExit, exitCode, "Exit code mismatch")

			output := buf.String()
			for _, expectedStr := range tc.expectOutput {
				assert.Contains(t, output, expectedStr, "Output mismatch")
			}
		})
	}
}

func TestHealJob(t *testing.T) {
	tests := []struct {
		name          string
		jobID         string
		jobResponse   interface{} // response body
		jobStatusCode int
		expectedExit  int
		expectOutput  []string
		wait          bool
	}{
		{
			name:          "Success",
			jobID:         "TEST-123",
			jobResponse:   map[string]string{"healed_job_id": "TEST-123-healed"},
			jobStatusCode: http.StatusAccepted,
			expectedExit:  -1, // No exit called
			expectOutput: []string{
				"Healing job TEST-123...",
				"Healed job TEST-123-healed submitted successfully.",
			},
		},
		{
			name:          "Job Not Found",
			jobID:         "TEST-404",
			jobResponse:   "job not found",
			jobStatusCode: http.StatusNotFound,
			expectedExit:  1,
			expectOutput: []string{
				"Healing job TEST-404...",
				"Failed to submit healed job: job not found",
			},
		},
		{
			name:          "Error connection failed",
			jobID:         "TEST-CONN-FAIL",
			jobResponse:   "",
			jobStatusCode: -1,
			expectedExit:  1,
			expectOutput: []string{
				"Failed to connect to orchestrator",
			},
		},
		{
			name:          "Error get invalid url",
			jobID:         "TEST-URL-FAIL",
			jobResponse:   "",
			jobStatusCode: -2,
			expectedExit:  1,
			expectOutput: []string{
				"Failed to create request",
			},
		},
		{
			name:          "Wait for job fails",
			jobID:         "TEST-WAIT",
			jobResponse:   map[string]string{"healed_job_id": "TEST-WAIT-healed"},
			jobStatusCode: http.StatusAccepted,
			expectedExit:  1,
			expectOutput: []string{
				"Healed job TEST-WAIT-healed submitted successfully.",
				"Healed job failed",
			},
			wait: true,
		},
		{
			name:          "Wait for job success",
			jobID:         "TEST-WAIT-SUCCESS",
			jobResponse:   map[string]string{"healed_job_id": "TEST-WAIT-SUCCESS-healed"},
			jobStatusCode: http.StatusAccepted,
			expectedExit:  -1,
			expectOutput: []string{
				"Healed job TEST-WAIT-SUCCESS-healed submitted successfully.",
			},
			wait: true,
		},
		{
			name:          "Invalid JSON in job details",
			jobID:         "TEST-BAD-JSON",
			jobResponse:   "invalid json string",
			jobStatusCode: http.StatusAccepted,
			expectedExit:  1,
			expectOutput: []string{
				"Failed to decode response:",
			},
		},
		{
			name:          "Submit API Error",
			jobID:         "TEST-FAIL",
			jobResponse:   "Invalid payload",
			jobStatusCode: http.StatusBadRequest,
			expectedExit:  1,
			expectOutput: []string{
				"Failed to submit healed job: Invalid payload",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/jobs/TEST-WAIT-healed" {
					json.NewEncoder(w).Encode(orchestrator.JobInfo{
						ID:     "TEST-WAIT-healed",
						Status: "Failed",
					})
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == "/jobs/TEST-WAIT-SUCCESS-healed" {
					json.NewEncoder(w).Encode(orchestrator.JobInfo{
						ID:     "TEST-WAIT-SUCCESS-healed",
						Status: "Completed",
					})
					return
				}

				if r.Method == http.MethodPost && r.URL.Path == "/jobs/"+tc.jobID+"/heal" {
					w.WriteHeader(tc.jobStatusCode)
					if strResp, ok := tc.jobResponse.(string); ok {
						w.Write([]byte(strResp))
					} else {
						json.NewEncoder(w).Encode(tc.jobResponse)
					}
					return
				}

				t.Fatalf("Unexpected request: %s %s", r.Method, r.URL.Path)
			}))
			defer server.Close()

			var buf bytes.Buffer
			originalStdout := stdout
			stdout = &buf
			defer func() { stdout = originalStdout }()

			originalExitFunc := exitFunc
			exitCode := -1
			exitFunc = func(code int) {
				exitCode = code
			}
			defer func() { exitFunc = originalExitFunc }()

			if tc.jobStatusCode == -2 {
				healJob("http://[::1]:namedport\n", tc.jobID, tc.wait)
			} else if tc.jobStatusCode == -1 {
				server.Close()
				healJob(server.URL, tc.jobID, tc.wait)
			} else {
				healJob(server.URL, tc.jobID, tc.wait)
			}

			assert.Equal(t, tc.expectedExit, exitCode, "Exit code mismatch")

			output := buf.String()
			for _, expectedStr := range tc.expectOutput {
				assert.Contains(t, output, expectedStr, "Output mismatch")
			}
		})
	}
}
