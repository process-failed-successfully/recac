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

			healJob(server.URL, tc.jobID, false)

			assert.Equal(t, tc.expectedExit, exitCode, "Exit code mismatch")

			output := buf.String()
			for _, expectedStr := range tc.expectOutput {
				assert.Contains(t, output, expectedStr, "Output mismatch")
			}
		})
	}
}
