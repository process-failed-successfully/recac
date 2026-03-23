package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
)

func TestGetJobOutput(t *testing.T) {
	tests := []struct {
		name           string
		jobID          string
		key            string
		mockResponse   interface{}
		mockStatus     int
		expectedOutput string
		expectExitCode int
	}{
		{
			name:  "Success Specific Key",
			jobID: "JOB-1",
			key:   "API_TOKEN",
			mockResponse: orchestrator.JobInfo{
				ID: "JOB-1",
				Outputs: map[string]string{
					"API_TOKEN": "secret123",
				},
			},
			mockStatus:     http.StatusOK,
			expectedOutput: "secret123\n",
			expectExitCode: 0,
		},
		{
			name:  "Success All JSON",
			jobID: "JOB-1",
			key:   "",
			mockResponse: orchestrator.JobInfo{
				ID: "JOB-1",
				Outputs: map[string]string{
					"API_TOKEN": "secret123",
					"USER_ID":   "99",
				},
			},
			mockStatus:     http.StatusOK,
			expectedOutput: "{\n  \"API_TOKEN\": \"secret123\",\n  \"USER_ID\": \"99\"\n}\n",
			expectExitCode: 0,
		},
		{
			name:  "Success Empty JSON",
			jobID: "JOB-1",
			key:   "",
			mockResponse: orchestrator.JobInfo{
				ID:      "JOB-1",
				Outputs: map[string]string{},
			},
			mockStatus:     http.StatusOK,
			expectedOutput: "{}\n",
			expectExitCode: 0,
		},
		{
			name:  "Success Nil Outputs JSON",
			jobID: "JOB-1",
			key:   "",
			mockResponse: orchestrator.JobInfo{
				ID:      "JOB-1",
				Outputs: nil,
			},
			mockStatus:     http.StatusOK,
			expectedOutput: "{}\n",
			expectExitCode: 0,
		},
		{
			name:  "Error Key Not Found",
			jobID: "JOB-1",
			key:   "MISSING_KEY",
			mockResponse: orchestrator.JobInfo{
				ID: "JOB-1",
				Outputs: map[string]string{
					"API_TOKEN": "secret123",
				},
			},
			mockStatus:     http.StatusOK,
			expectedOutput: "Error: Output key 'MISSING_KEY' not found for job JOB-1\n",
			expectExitCode: 1,
		},
		{
			name:           "Error Job Not Found",
			jobID:          "JOB-MISSING",
			key:            "",
			mockResponse:   "job not found",
			mockStatus:     http.StatusNotFound,
			expectedOutput: "Failed to fetch job details: \"job not found\"\n",
			expectExitCode: 1,
		},
		{
			name:           "Error Invalid JSON",
			jobID:          "JOB-1",
			key:            "",
			mockResponse:   "invalid json",
			mockStatus:     http.StatusOK, // but body is bad
			expectedOutput: "Failed to decode response: invalid character 'i' looking for beginning of value\n",
			expectExitCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			stdout = &out
			defer func() { stdout = nil }()

			exitCode := 0
			exitFunc = func(code int) {
				exitCode = code
			}
			defer func() { exitFunc = nil }()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/jobs/"+tt.jobID, r.URL.Path)

				w.WriteHeader(tt.mockStatus)
				if tt.name == "Error Invalid JSON" {
					w.Write([]byte(tt.mockResponse.(string)))
				} else if strResp, ok := tt.mockResponse.(string); ok {
					// Just write string
					w.Write([]byte(`"` + strResp + `"`))
				} else {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			getJobOutput(server.URL, tt.jobID, tt.key)

			assert.Equal(t, tt.expectExitCode, exitCode)
			assert.Equal(t, tt.expectedOutput, out.String())
		})
	}
}

func TestGetJobOutput_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	stdout = &out
	defer func() { stdout = nil }()

	exitCode := 0
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = nil }()

	getJobOutput("http://localhost:0", "JOB-1", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator at http://localhost:0")
}
