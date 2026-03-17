package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		jobResponse   interface{} // orchestrator.JobInfo or string
		jobStatusCode int
		logResponse   string
		logStatusCode int
		postHandler   func(w http.ResponseWriter, r *http.Request)
		expectedExit  int
		expectOutput  []string
		wait          bool
	}{
		{
			name:  "Success",
			jobID: "TEST-123",
			jobResponse: orchestrator.JobInfo{
				ID:      "TEST-123",
				Status:  "Failed",
				Error:   "Syntax error on line 42",
				Summary: "Fix bug",
				WorkItem: orchestrator.WorkItem{
					ID:          "TEST-123",
					Summary:     "Fix bug",
					Description: "Original description",
					Tags:        []string{"bug"},
				},
			},
			jobStatusCode: http.StatusOK,
			logResponse:   "Line 1\nLine 2\nError: Syntax error on line 42",
			logStatusCode: http.StatusOK,
			postHandler: func(w http.ResponseWriter, r *http.Request) {
				var submitted orchestrator.WorkItem
				err := json.NewDecoder(r.Body).Decode(&submitted)
				require.NoError(t, err)

				assert.Equal(t, "TEST-123-healed", submitted.ID)
				assert.Contains(t, submitted.Description, "Original description")
				assert.Contains(t, submitted.Description, "Previous Job Failure Context:")
				assert.Contains(t, submitted.Description, "Syntax error on line 42")
				assert.Contains(t, submitted.Tags, "auto-heal")
				assert.Contains(t, submitted.Tags, "bug")

				w.WriteHeader(http.StatusAccepted)
				w.Write([]byte("Job TEST-123-healed submitted successfully"))
			},
			expectedExit: -1, // No exit called
			expectOutput: []string{
				"Fetching job details for TEST-123...",
				"Fetching logs for TEST-123...",
				"Submitting healed job TEST-123-healed...",
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
				"Fetching job details for TEST-404...",
				"Failed to fetch job details: job not found",
			},
		},
		{
			name:  "Logs API Error (Still creates healed job)",
			jobID: "TEST-NOLOGS",
			jobResponse: orchestrator.JobInfo{
				ID:      "TEST-NOLOGS",
				Status:  "Failed",
				Error:   "Unknown error",
				Summary: "Fix bug",
				WorkItem: orchestrator.WorkItem{
					ID:          "TEST-NOLOGS",
					Summary:     "Fix bug",
					Description: "Original description",
				},
			},
			jobStatusCode: http.StatusOK,
			logResponse:   "logs not available",
			logStatusCode: http.StatusNotFound,
			postHandler: func(w http.ResponseWriter, r *http.Request) {
				var submitted orchestrator.WorkItem
				err := json.NewDecoder(r.Body).Decode(&submitted)
				require.NoError(t, err)

				assert.Equal(t, "TEST-NOLOGS-healed", submitted.ID)
				assert.Contains(t, submitted.Description, "Unknown error")
				// Empty logs should still be in the failure context block
				assert.Contains(t, submitted.Description, "Previous Job Failure Context")

				w.WriteHeader(http.StatusAccepted)
				w.Write([]byte("Job submitted"))
			},
			expectedExit: -1,
			expectOutput: []string{
				"Fetching job details for TEST-NOLOGS...",
				"Fetching logs for TEST-NOLOGS...",
				"Warning: Failed to fetch logs, status 404",
				"Submitting healed job TEST-NOLOGS-healed...",
				"Healed job TEST-NOLOGS-healed submitted successfully.",
			},
		},
		{
			name:  "Error connection failed",
			jobID: "TEST-CONN-FAIL",
			jobResponse: orchestrator.JobInfo{
				ID: "TEST-CONN-FAIL",
			},
			jobStatusCode: -1,
			expectedExit:  1,
			expectOutput: []string{
				"Failed to connect to orchestrator",
			},
		},
		{
			name:  "Error get invalid url",
			jobID: "TEST-URL-FAIL",
			jobResponse: orchestrator.JobInfo{
				ID: "TEST-URL-FAIL",
			},
			jobStatusCode: -2,
			expectedExit:  1,
			expectOutput: []string{
				"Failed to connect to orchestrator",
			},
		},
		{
			name:  "Wait for job fails",
			jobID: "TEST-WAIT",
			jobResponse: orchestrator.JobInfo{
				ID:      "TEST-WAIT",
				Status:  "Failed",
				Error:   "Unknown error",
				Summary: "Fix bug",
				WorkItem: orchestrator.WorkItem{
					ID:          "TEST-WAIT",
					Summary:     "Fix bug",
					Description: "Original description",
				},
			},
			jobStatusCode: http.StatusOK,
			logResponse:   "logs available",
			logStatusCode: http.StatusOK,
			postHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
				w.Write([]byte("Job submitted"))
			},
			expectedExit: 1,
			expectOutput: []string{
				"Submitting healed job TEST-WAIT-healed...",
				"Healed job TEST-WAIT-healed submitted successfully.",
				"Healed job failed",
			},
			wait: true,
		},
		{
			name:  "Wait for job success",
			jobID: "TEST-WAIT-SUCCESS",
			jobResponse: orchestrator.JobInfo{
				ID:      "TEST-WAIT-SUCCESS",
				Status:  "Failed",
				Error:   "Unknown error",
				Summary: "Fix bug",
				WorkItem: orchestrator.WorkItem{
					ID:          "TEST-WAIT-SUCCESS",
					Summary:     "Fix bug",
					Description: "Original description",
				},
			},
			jobStatusCode: http.StatusOK,
			logResponse:   "logs available",
			logStatusCode: http.StatusOK,
			postHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
				w.Write([]byte("Job submitted"))
			},
			expectedExit: -1,
			expectOutput: []string{
				"Submitting healed job TEST-WAIT-SUCCESS-healed...",
				"Healed job TEST-WAIT-SUCCESS-healed submitted successfully.",
			},
			wait: true,
		},
		{
			name:  "Invalid JSON in job details",
			jobID: "TEST-BAD-JSON",
			jobResponse: "invalid json string",
			jobStatusCode: http.StatusOK,
			expectedExit:  1,
			expectOutput: []string{
				"Failed to decode response:",
			},
		},
		{
			name:  "Submit API Error",
			jobID: "TEST-FAIL",
			jobResponse: orchestrator.JobInfo{
				ID:      "TEST-FAIL",
				Status:  "Failed",
				Error:   "error",
				WorkItem: orchestrator.WorkItem{ID: "TEST-FAIL"},
			},
			jobStatusCode: http.StatusOK,
			logResponse:   "logs",
			logStatusCode: http.StatusOK,
			postHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Invalid payload"))
			},
			expectedExit: 1,
			expectOutput: []string{
				"Submitting healed job TEST-FAIL-healed...",
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
				if r.Method == http.MethodGet && r.URL.Path == "/jobs/"+tc.jobID {
					w.WriteHeader(tc.jobStatusCode)
					if strResp, ok := tc.jobResponse.(string); ok {
						w.Write([]byte(strResp))
					} else {
						json.NewEncoder(w).Encode(tc.jobResponse)
					}
					return
				}

				if r.Method == http.MethodGet && r.URL.Path == "/jobs/"+tc.jobID+"/logs" {
					w.WriteHeader(tc.logStatusCode)
					w.Write([]byte(tc.logResponse))
					return
				}

				if r.Method == http.MethodPost && r.URL.Path == "/jobs" {
					if tc.postHandler != nil {
						tc.postHandler(w, r)
					} else {
						t.Fatalf("Unexpected POST to /jobs")
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
				healJob("http://[::1]:namedport", tc.jobID, tc.wait)
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
