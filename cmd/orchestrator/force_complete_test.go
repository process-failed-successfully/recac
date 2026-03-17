package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestForceCompleteJobCLI(t *testing.T) {
	// Setup mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/job123/force-complete" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "Job job123 force completed")
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// Capture stdout
	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() {
		stdout = oldStdout
	}()

	// Run the client function
	forceCompleteJob(ts.URL, "job123")
	w.Close()

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Job job123 force completed successfully.") {
		t.Errorf("Expected success message, got: %s", output)
	}
}

func TestForceCompleteBulkJobsCLI(t *testing.T) {
	// Setup mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/force-complete" && r.Method == http.MethodPost {
			tag := r.URL.Query().Get("tag")
			match := r.URL.Query().Get("match")
			if tag == "mytag" && match == "" {
				w.WriteHeader(http.StatusOK)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintln(w, `{"force_completed": 5}`)
				return
			}
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	// Capture stdout
	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() {
		stdout = oldStdout
	}()

	// Run the client function
	forceCompleteBulkJobs(ts.URL, "", "mytag")
	w.Close()

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Successfully force completed 5 jobs.") {
		t.Errorf("Expected success message, got: %s", output)
	}
}
