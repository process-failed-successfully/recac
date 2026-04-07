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

func TestApproveInteractive(t *testing.T) {
	tests := []struct {
		name            string
		jobs            []orchestrator.JobInfo
		inputStr        string
		expectedOutput  []string
		expectedExit    int
		expectedMethods map[string]string // URL Path -> HTTP Method expected
	}{
		{
			name: "No jobs pending approval",
			jobs: []orchestrator.JobInfo{
				{ID: "JOB-1", Status: "Running"},
			},
			inputStr: "",
			expectedOutput: []string{
				"No jobs are currently pending approval.",
			},
			expectedExit: 0,
		},
		{
			name: "Approve a job",
			jobs: []orchestrator.JobInfo{
				{ID: "JOB-1", Status: "Pending Approval", Summary: "Summary 1"},
			},
			inputStr: "y\n",
			expectedOutput: []string{
				"Interactive Approval (1 jobs)",
				"ID:", "JOB-1",
				"Summary:", "Summary 1",
				"Action for JOB-1",
				"Job JOB-1 approved successfully.",
				"All pending approval jobs processed.",
			},
			expectedExit: 0,
			expectedMethods: map[string]string{
				"/jobs/JOB-1/approve": http.MethodPost,
			},
		},
		{
			name: "Skip a job",
			jobs: []orchestrator.JobInfo{
				{ID: "JOB-1", Status: "Pending Approval", Summary: "Summary 1"},
			},
			inputStr: "s\n",
			expectedOutput: []string{
				"Interactive Approval (1 jobs)",
				"ID:", "JOB-1",
				"Summary:", "Summary 1",
				"Action for JOB-1",
				"Job JOB-1 skipped successfully.",
				"All pending approval jobs processed.",
			},
			expectedExit: 0,
			expectedMethods: map[string]string{
				"/jobs/JOB-1/skip": http.MethodPost,
			},
		},
		{
			name: "Cancel a job",
			jobs: []orchestrator.JobInfo{
				{ID: "JOB-1", Status: "Pending Approval", Summary: "Summary 1"},
			},
			inputStr: "c\n",
			expectedOutput: []string{
				"Interactive Approval (1 jobs)",
				"ID:", "JOB-1",
				"Summary:", "Summary 1",
				"Action for JOB-1",
				"Job JOB-1 cancelled successfully.",
				"All pending approval jobs processed.",
			},
			expectedExit: 0,
			expectedMethods: map[string]string{
				"/jobs/JOB-1": http.MethodDelete,
			},
		},
		{
			name: "Quit early",
			jobs: []orchestrator.JobInfo{
				{ID: "JOB-1", Status: "Pending Approval", Summary: "Summary 1"},
				{ID: "JOB-2", Status: "Pending Approval", Summary: "Summary 2"},
			},
			inputStr: "q\n",
			expectedOutput: []string{
				"Interactive Approval (2 jobs)",
				"ID:", "JOB-1",
				"Summary:", "Summary 1",
				"Action for JOB-1",
				"Exiting interactive approval.",
			},
			expectedExit: 0,
		},
		{
			name: "Invalid input then approve",
			jobs: []orchestrator.JobInfo{
				{ID: "JOB-1", Status: "Pending Approval", Summary: "Summary 1"},
			},
			inputStr: "invalid\ny\n",
			expectedOutput: []string{
				"Interactive Approval (1 jobs)",
				"ID:", "JOB-1",
				"Invalid input. Please enter 'a', 's', 'c', or 'q'.",
				"Job JOB-1 approved successfully.",
			},
			expectedExit: 0,
			expectedMethods: map[string]string{
				"/jobs/JOB-1/approve": http.MethodPost,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			methodsCalled := make(map[string]string)

			// Mock Server
			mux := http.NewServeMux()
			mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "pending", r.URL.Query().Get("state"))
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(tt.jobs)
			})

			// Mock the action endpoints
			actionHandler := func(w http.ResponseWriter, r *http.Request) {
				methodsCalled[r.URL.Path] = r.Method
				w.WriteHeader(http.StatusOK)
				if r.Method == http.MethodDelete {
					w.Write([]byte(`{"canceled_jobs": []}`))
				} else {
					w.Write([]byte(`{"message": "success"}`))
				}
			}

			mux.HandleFunc("/jobs/JOB-1/approve", actionHandler)
			mux.HandleFunc("/jobs/JOB-1/skip", actionHandler)
			mux.HandleFunc("/jobs/JOB-1", actionHandler) // Cancel uses DELETE /jobs/{id}

			server := httptest.NewServer(mux)
			defer server.Close()

			// Mock stdout
			var buf bytes.Buffer
			oldStdout := stdout
			stdout = &buf
			defer func() { stdout = oldStdout }()

			// Mock stdin
			oldStdin := stdin
			stdin = strings.NewReader(tt.inputStr)
			defer func() { stdin = oldStdin }()

			// Mock exitFunc
			oldExit := exitFunc
			exitCode := 0
			exitFunc = func(code int) { exitCode = code }
			defer func() { exitFunc = oldExit }()

			approveInteractive(server.URL)

			assert.Equal(t, tt.expectedExit, exitCode)

			output := buf.String()
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, output, expected)
			}

			for path, expectedMethod := range tt.expectedMethods {
				assert.Equal(t, expectedMethod, methodsCalled[path], "Expected method %s for path %s", expectedMethod, path)
			}
		})
	}
}
