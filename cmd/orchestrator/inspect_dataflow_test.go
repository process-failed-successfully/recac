package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
)

func TestInspectDataflow(t *testing.T) {
	tests := []struct {
		name             string
		jobID            string
		mockResponses    map[string]interface{}
		mockStatuses     map[string]int
		expectedContains []string
		expectExitCode   int
	}{
		{
			name:  "Success With Dataflow",
			jobID: "JOB-TARGET",
			mockResponses: map[string]interface{}{
				"/jobs/JOB-TARGET": orchestrator.JobInfo{
					ID: "JOB-TARGET",
					WorkItem: orchestrator.WorkItem{
						DependsOn: []string{"JOB-DEP-1", "JOB-DEP-2"},
					},
				},
				"/jobs/JOB-DEP-1": orchestrator.JobInfo{
					ID: "JOB-DEP-1",
					Outputs: map[string]string{
						"BUILD_DIR": "/out/build",
					},
				},
				"/jobs/JOB-DEP-2": orchestrator.JobInfo{
					ID: "JOB-DEP-2",
					Outputs: map[string]string{
						"status": "ok",
					},
				},
			},
			mockStatuses: map[string]int{
				"/jobs/JOB-TARGET": http.StatusOK,
				"/jobs/JOB-DEP-1":  http.StatusOK,
				"/jobs/JOB-DEP-2":  http.StatusOK,
			},
			expectedContains: []string{
				"Dataflow Inspection: JOB-TARGET",
				"From JOB-DEP-1",
				"DEP_JOB_DEP_1_BUILD_DIR=/out/build",
				"From JOB-DEP-2",
				"DEP_JOB_DEP_2_STATUS=ok",
			},
			expectExitCode: 0,
		},
		{
			name:  "Success No Dependencies",
			jobID: "JOB-TARGET",
			mockResponses: map[string]interface{}{
				"/jobs/JOB-TARGET": orchestrator.JobInfo{
					ID: "JOB-TARGET",
					WorkItem: orchestrator.WorkItem{
						DependsOn: []string{},
					},
				},
			},
			mockStatuses: map[string]int{
				"/jobs/JOB-TARGET": http.StatusOK,
			},
			expectedContains: []string{
				"Job JOB-TARGET has no dependencies.",
			},
			expectExitCode: 0,
		},
		{
			name:  "Success Dep Has No Outputs",
			jobID: "JOB-TARGET",
			mockResponses: map[string]interface{}{
				"/jobs/JOB-TARGET": orchestrator.JobInfo{
					ID: "JOB-TARGET",
					WorkItem: orchestrator.WorkItem{
						DependsOn: []string{"JOB-DEP-1"},
					},
				},
				"/jobs/JOB-DEP-1": orchestrator.JobInfo{
					ID:      "JOB-DEP-1",
					Outputs: map[string]string{},
				},
			},
			mockStatuses: map[string]int{
				"/jobs/JOB-TARGET": http.StatusOK,
				"/jobs/JOB-DEP-1":  http.StatusOK,
			},
			expectedContains: []string{
				"No outputs generated.",
				"No dataflow variables injected",
			},
			expectExitCode: 0,
		},
		{
			name:  "Error Target Job Not Found",
			jobID: "JOB-MISSING",
			mockResponses: map[string]interface{}{
				"/jobs/JOB-MISSING": "not found",
			},
			mockStatuses: map[string]int{
				"/jobs/JOB-MISSING": http.StatusNotFound,
			},
			expectedContains: []string{
				"Failed to fetch target job details",
			},
			expectExitCode: 1,
		},
		{
			name:  "Dependency Missing",
			jobID: "JOB-TARGET",
			mockResponses: map[string]interface{}{
				"/jobs/JOB-TARGET": orchestrator.JobInfo{
					ID: "JOB-TARGET",
					WorkItem: orchestrator.WorkItem{
						DependsOn: []string{"JOB-DEP-MISSING"},
					},
				},
				"/jobs/JOB-DEP-MISSING": "not found",
			},
			mockStatuses: map[string]int{
				"/jobs/JOB-TARGET":      http.StatusOK,
				"/jobs/JOB-DEP-MISSING": http.StatusNotFound,
			},
			expectedContains: []string{
				"Dependency JOB-DEP-MISSING:",
				"Could not fetch details.",
			},
			expectExitCode: 0,
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
				path := r.URL.Path

				status, ok := tt.mockStatuses[path]
				if !ok {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				resp, ok := tt.mockResponses[path]
				if !ok {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				w.WriteHeader(status)
				if strResp, ok := resp.(string); ok {
					w.Write([]byte(strResp))
				} else {
					json.NewEncoder(w).Encode(resp)
				}
			}))
			defer server.Close()

			inspectDataflow(server.URL, tt.jobID)

			assert.Equal(t, tt.expectExitCode, exitCode)

			outputStr := out.String()
			for _, expected := range tt.expectedContains {
				if strings.Contains(expected, "\x1b") {
					assert.Contains(t, outputStr, expected) // literal ansi color check
				} else {
					// strip ansi for regular checks to make them less brittle
					stripped := stripAnsi(outputStr)
					assert.Contains(t, stripped, expected)
				}
			}
		})
	}
}

func TestInspectDataflow_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	stdout = &out
	defer func() { stdout = nil }()

	exitCode := 0
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = nil }()

	inspectDataflow("http://localhost:0", "JOB-1")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator at http://localhost:0")
}

func stripAnsi(str string) string {
	var buf strings.Builder
	inEscape := false
	for _, r := range str {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		buf.WriteRune(r)
	}
	return buf.String()
}
