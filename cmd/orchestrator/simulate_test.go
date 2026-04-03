package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

//
func TestSimulateExecution_TableDriven(t *testing.T) {
	origExit := exitFunc
	defer func() { exitFunc = origExit }()
	exitFunc = func(int) {}

	origStdout := stdout
	defer func() { stdout = origStdout }()

	tests := []struct {
		name           string
		responseJSON   string
		responseStatus int
		expectContains []string
	}{
		{
			name: "Success without deadlocks",
			responseJSON: `{"total_jobs": 5, "jobs_processed": 5, "estimated_total_time_ms": 10000, "deadlocks": 0}`,
			responseStatus: http.StatusOK,
			expectContains: []string{"Simulation Report", "Jobs Processed:", "Estimated Total Time"},
		},
		{
			name: "Success with deadlocks",
			responseJSON: `{"total_jobs": 5, "jobs_processed": 3, "estimated_total_time_ms": 5000, "deadlocks": 2}`,
			responseStatus: http.StatusOK,
			expectContains: []string{"WARNING:", "2 jobs could not be processed"},
		},
		{
			name: "No jobs to simulate",
			responseJSON: `{"total_jobs": 0, "jobs_processed": 0, "estimated_total_time_ms": 0, "deadlocks": 0}`,
			responseStatus: http.StatusOK,
			expectContains: []string{"No active or pending jobs"},
		},
		{
			name: "Server error",
			responseJSON: `Internal Server Error`,
			responseStatus: http.StatusInternalServerError,
			expectContains: []string{"Failed to fetch simulation report"},
		},
		{
			name: "Bad JSON",
			responseJSON: `{bad json}`,
			responseStatus: http.StatusOK,
			expectContains: []string{"Failed to decode response:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			stdout = &buf

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/simulate" {
					w.WriteHeader(tt.responseStatus)
					w.Write([]byte(tt.responseJSON))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()





			simulateExecution(server.URL)

			for _, exp := range tt.expectContains {
				assert.Contains(t, buf.String(), exp)
			}
		})
	}
}
