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

func TestForceCompleteJobCLI_Errors(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() {
		exitFunc = oldExit
	}()

	oldStdout := stdout
	_, w, _ := os.Pipe()
	stdout = w
	defer func() {
		stdout = oldStdout
	}()

	t.Run("Server Unreachable", func(t *testing.T) {
		exitCode = 0
		forceCompleteJob("http://127.0.0.1:0", "job123")
		if exitCode != 1 {
			t.Errorf("Expected exit code 1 for server unreachable, got %d", exitCode)
		}
	})

	t.Run("Non-200 OK", func(t *testing.T) {
		exitCode = 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, "internal server error")
		}))
		defer ts.Close()

		forceCompleteJob(ts.URL, "job123")
		if exitCode != 1 {
			t.Errorf("Expected exit code 1 for non-200 response, got %d", exitCode)
		}
	})

	t.Run("Invalid URL", func(t *testing.T) {
		exitCode = 0
		forceCompleteJob("http://[fe80::1%en0]:8080", "job123")
		if exitCode != 1 {
			t.Errorf("Expected exit code 1 for invalid URL, got %d", exitCode)
		}
	})
}

func TestForceCompleteBulkJobsCLI_Errors(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() {
		exitFunc = oldExit
	}()

	oldStdout := stdout
	_, w, _ := os.Pipe()
	stdout = w
	defer func() {
		stdout = oldStdout
	}()

	t.Run("Server Unreachable", func(t *testing.T) {
		exitCode = 0
		forceCompleteBulkJobs("http://127.0.0.1:0", "", "")
		if exitCode != 1 {
			t.Errorf("Expected exit code 1 for server unreachable, got %d", exitCode)
		}
	})

	t.Run("Non-200 OK", func(t *testing.T) {
		exitCode = 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "bad request")
		}))
		defer ts.Close()

		forceCompleteBulkJobs(ts.URL, "test", "")
		if exitCode != 1 {
			t.Errorf("Expected exit code 1 for non-200 response, got %d", exitCode)
		}
	})

	t.Run("JSON Decode Error", func(t *testing.T) {
		exitCode = 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "invalid json")
		}))
		defer ts.Close()

		forceCompleteBulkJobs(ts.URL, "", "test")
		if exitCode != 1 {
			t.Errorf("Expected exit code 1 for json decode error, got %d", exitCode)
		}
	})

	t.Run("Invalid URL", func(t *testing.T) {
		exitCode = 0
		forceCompleteBulkJobs("http://[fe80::1%en0]:8080", "", "")
		if exitCode != 1 {
			t.Errorf("Expected exit code 1 for invalid URL, got %d", exitCode)
		}
	})
}

func TestForceCompleteBulkJobsCLI_RequestError(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() {
		exitFunc = oldExit
	}()

	oldStdout := stdout
	_, w, _ := os.Pipe()
	stdout = w
	defer func() {
		stdout = oldStdout
	}()

	exitCode = 0
	forceCompleteBulkJobs("http://[::1]:namedport", "", "")
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for url parse error, got %d", exitCode)
	}

	exitCode = 0
	forceCompleteBulkJobs("http://127.0.0.1:8080\x7f", "", "")
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for request error, got %d", exitCode)
	}
}
