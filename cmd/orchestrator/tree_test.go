package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"recac/internal/orchestrator"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestPrintTree(t *testing.T) {
	tests := []struct {
		name     string
		jobs     []orchestrator.JobInfo
		expected []string
	}{
		{
			name: "No Jobs",
			jobs: []orchestrator.JobInfo{},
			expected: []string{
				"No jobs found.",
			},
		},
		{
			name: "Disconnected Jobs",
			jobs: []orchestrator.JobInfo{
				{ID: "JOB-1", Status: "Completed", Summary: "Job One"},
				{ID: "JOB-2", Status: "Pending", Summary: "Job Two"},
			},
			expected: []string{
				"Job Dependency Tree (2 Jobs)",
				"JOB-1",
				"JOB-2",
				"Completed",
				"Pending",
				"Job One",
				"Job Two",
				"└──", // Both are roots and last in their own single-node tree
			},
		},
		{
			name: "Linear Chain",
			jobs: []orchestrator.JobInfo{
				{ID: "JOB-1", Status: "Completed", Summary: "Job One", WorkItem: orchestrator.WorkItem{DependsOn: []string{}}},
				{ID: "JOB-2", Status: "Active", Summary: "Job Two", WorkItem: orchestrator.WorkItem{DependsOn: []string{"JOB-1"}}},
				{ID: "JOB-3", Status: "Pending", Summary: "Job Three", WorkItem: orchestrator.WorkItem{DependsOn: []string{"JOB-2"}}},
			},
			expected: []string{
				"Job Dependency Tree (3 Jobs)",
				"JOB-1",
				"JOB-2",
				"JOB-3",
				"└──",
				"    └──",
			},
		},
		{
			name: "Complex Tree",
			jobs: []orchestrator.JobInfo{
				{ID: "JOB-1", Status: "Completed", Summary: "Job One"},
				{ID: "JOB-2", Status: "Completed", Summary: "Job Two", WorkItem: orchestrator.WorkItem{DependsOn: []string{"JOB-1"}}},
				{ID: "JOB-3", Status: "Failed", Summary: "Job Three", WorkItem: orchestrator.WorkItem{DependsOn: []string{"JOB-1"}}},
				{ID: "JOB-4", Status: "Pending", Summary: "Job Four", WorkItem: orchestrator.WorkItem{DependsOn: []string{"JOB-2", "JOB-3"}}},
			},
			expected: []string{
				"Job Dependency Tree (4 Jobs)",
				"JOB-1",
				"JOB-2",
				"JOB-3",
				"JOB-4",
				"├──",
				"└──",
			},
		},
		{
			name: "Missing Dependency",
			jobs: []orchestrator.JobInfo{
				{ID: "JOB-2", Status: "Pending", Summary: "Job Two", WorkItem: orchestrator.WorkItem{DependsOn: []string{"JOB-1"}}},
			},
			expected: []string{
				"Job Dependency Tree (1 Jobs)",
				"JOB-2",
				"└──",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock server
			mux := http.NewServeMux()
			mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "all", r.URL.Query().Get("state"))
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tt.jobs)
			})

			server := httptest.NewServer(mux)
			defer server.Close()

			// Redirect stdout
			var out bytes.Buffer
			oldStdout := stdout
			stdout = &out
			defer func() { stdout = oldStdout }()

			// Set up viper
			viper.Reset()
			viper.Set("orchestrator.tree", true)
			viper.Set("orchestrator.host", server.URL)

			// Mock exitFunc
			oldExitFunc := exitFunc
			exitFunc = func(code int) {}
			defer func() { exitFunc = oldExitFunc }()

			// Execute
			printTree(server.URL)

			// Verify
			output := out.String()
			for _, exp := range tt.expected {
				assert.Contains(t, output, exp)
			}
		})
	}
}

func TestTreeCommandFlag(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]orchestrator.JobInfo{
			{ID: "JOB-1", Status: "Completed", Summary: "Test Job"},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	viper.Reset()
	defer viper.Reset()

	viper.Set("orchestrator.tree", true)
	viper.Set("orchestrator.host", server.URL)

	// Disable other flags to isolate the test
	viper.Set("orchestrator.list_jobs", false)
	viper.Set("orchestrator.status", false)
	viper.Set("orchestrator.cancel_all", false)
	viper.Set("orchestrator.retry_failed", false)
	viper.Set("orchestrator.pause", false)
	viper.Set("orchestrator.resume", false)
	viper.Set("orchestrator.monitor", false)
	viper.Set("orchestrator.scale", -1)

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExitFunc := exitFunc
	exitFunc = func(code int) {}
	defer func() { exitFunc = oldExitFunc }()

	// Calling run will hit the tree flag and call printTree
	err := run(nil, nil)
	assert.NoError(t, err)

	assert.Contains(t, out.String(), "Job Dependency Tree")
	assert.Contains(t, out.String(), "JOB-1")
}
