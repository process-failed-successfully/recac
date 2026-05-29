package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeletePendingJob(t *testing.T) {
	tests := []struct {
		name         string
		jobID        string
		method       string
		path         string
		statusCode   int
		responseBody string
		clientError  bool
		urlError     bool
		expectExit   bool
		expectedOut  string
	}{
		{
			name:        "Success",
			jobID:       "test-job",
			method:      http.MethodDelete,
			path:        "/jobs/test-job/pending",
			statusCode:  http.StatusOK,
			expectExit:  false,
			expectedOut: "Pending job test-job deleted successfully.\n",
		},
		{
			name:         "HTTP Error",
			jobID:        "test-job",
			method:       http.MethodDelete,
			path:         "/jobs/test-job/pending",
			statusCode:   http.StatusInternalServerError,
			responseBody: "internal server error",
			expectExit:   true,
			expectedOut:  "Failed to delete pending job: internal server error\n",
		},
		{
			name:       "URL Error",
			jobID:      "test-job",
			urlError:   true,
			expectExit: true,
		},
		{
			name:        "Client Error",
			jobID:       "test-job",
			clientError: true,
			expectExit:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method {
					t.Errorf("Expected %s method, got %s", tt.method, r.Method)
				}
				if r.URL.Path != tt.path {
					t.Errorf("Expected %s path, got %s", tt.path, r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					fmt.Fprint(w, tt.responseBody)
				}
			}))
			defer ts.Close()

			var out bytes.Buffer
			oldStdout := stdout
			stdout = &out
			defer func() { stdout = oldStdout }()

			exitCalled := false
			oldExitFunc := exitFunc
			exitFunc = func(code int) { exitCalled = true }
			defer func() { exitFunc = oldExitFunc }()

			var host string
			if tt.urlError {
				host = "http://invalid\nurl"
			} else if tt.clientError {
				host = "http://localhost:0"
			} else {
				host = ts.URL
			}

			deletePendingJob(host, tt.jobID)

			if exitCalled != tt.expectExit {
				t.Errorf("Expected exit=%v, got %v", tt.expectExit, exitCalled)
			}
			if tt.expectedOut != "" && out.String() != tt.expectedOut {
				t.Errorf("Expected output %q, got %q", tt.expectedOut, out.String())
			}
		})
	}
}

func TestDeletePendingJobsByGroup(t *testing.T) {
	tests := []struct {
		name         string
		group        string
		method       string
		path         string
		statusCode   int
		responseBody string
		clientError  bool
		urlError     bool
		expectExit   bool
		expectedOut  string
	}{
		{
			name:         "Success",
			group:        "test-group",
			method:       http.MethodDelete,
			path:         "/jobs/pending",
			statusCode:   http.StatusOK,
			responseBody: `{"deleted": 2}`,
			expectExit:   false,
			expectedOut:  "Successfully deleted 2 pending jobs by concurrency group.\n",
		},
		{
			name:         "HTTP Error",
			group:        "test-group",
			method:       http.MethodDelete,
			path:         "/jobs/pending",
			statusCode:   http.StatusInternalServerError,
			responseBody: "internal server error",
			expectExit:   true,
			expectedOut:  "Failed to delete pending jobs by concurrency group: internal server error\n",
		},
		{
			name:         "JSON Decode Error",
			group:        "test-group",
			method:       http.MethodDelete,
			path:         "/jobs/pending",
			statusCode:   http.StatusOK,
			responseBody: `invalid json`,
			expectExit:   true,
		},
		{
			name:       "URL Error",
			group:      "test-group",
			urlError:   true,
			expectExit: true,
		},
		{
			name:        "Client Error",
			group:       "test-group",
			clientError: true,
			expectExit:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method {
					t.Errorf("Expected %s method, got %s", tt.method, r.Method)
				}
				if r.URL.Path != tt.path {
					t.Errorf("Expected %s path, got %s", tt.path, r.URL.Path)
				}
				if r.URL.Query().Get("group") != tt.group {
					t.Errorf("Expected group %s, got %s", tt.group, r.URL.Query().Get("group"))
				}
				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					fmt.Fprint(w, tt.responseBody)
				}
			}))
			defer ts.Close()

			var out bytes.Buffer
			oldStdout := stdout
			stdout = &out
			defer func() { stdout = oldStdout }()

			exitCalled := false
			oldExitFunc := exitFunc
			exitFunc = func(code int) { exitCalled = true }
			defer func() { exitFunc = oldExitFunc }()

			var host string
			if tt.urlError {
				host = "http://invalid\nurl"
			} else if tt.clientError {
				host = "http://localhost:0"
			} else {
				host = ts.URL
			}

			deletePendingJobsByGroup(host, tt.group)

			if exitCalled != tt.expectExit {
				t.Errorf("Expected exit=%v, got %v", tt.expectExit, exitCalled)
			}
			if tt.expectedOut != "" && out.String() != tt.expectedOut {
				t.Errorf("Expected output %q, got %q", tt.expectedOut, out.String())
			}
		})
	}
}

func TestDeletePendingJobsByTag(t *testing.T) {
	tests := []struct {
		name         string
		tag          string
		method       string
		path         string
		statusCode   int
		responseBody string
		clientError  bool
		urlError     bool
		expectExit   bool
		expectedOut  string
	}{
		{
			name:         "Success",
			tag:          "test-tag",
			method:       http.MethodDelete,
			path:         "/jobs/pending",
			statusCode:   http.StatusOK,
			responseBody: `{"deleted": 5}`,
			expectExit:   false,
			expectedOut:  "Successfully deleted 5 pending jobs by tag.\n",
		},
		{
			name:         "HTTP Error",
			tag:          "test-tag",
			method:       http.MethodDelete,
			path:         "/jobs/pending",
			statusCode:   http.StatusInternalServerError,
			responseBody: "internal server error",
			expectExit:   true,
			expectedOut:  "Failed to delete pending jobs by tag: internal server error\n",
		},
		{
			name:         "JSON Decode Error",
			tag:          "test-tag",
			method:       http.MethodDelete,
			path:         "/jobs/pending",
			statusCode:   http.StatusOK,
			responseBody: `invalid json`,
			expectExit:   true,
		},
		{
			name:       "URL Error",
			tag:        "test-tag",
			urlError:   true,
			expectExit: true,
		},
		{
			name:        "Client Error",
			tag:         "test-tag",
			clientError: true,
			expectExit:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method {
					t.Errorf("Expected %s method, got %s", tt.method, r.Method)
				}
				if r.URL.Path != tt.path {
					t.Errorf("Expected %s path, got %s", tt.path, r.URL.Path)
				}
				if r.URL.Query().Get("tag") != tt.tag {
					t.Errorf("Expected tag %s, got %s", tt.tag, r.URL.Query().Get("tag"))
				}
				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					fmt.Fprint(w, tt.responseBody)
				}
			}))
			defer ts.Close()

			var out bytes.Buffer
			oldStdout := stdout
			stdout = &out
			defer func() { stdout = oldStdout }()

			exitCalled := false
			oldExitFunc := exitFunc
			exitFunc = func(code int) { exitCalled = true }
			defer func() { exitFunc = oldExitFunc }()

			var host string
			if tt.urlError {
				host = "http://invalid\nurl"
			} else if tt.clientError {
				host = "http://localhost:0"
			} else {
				host = ts.URL
			}

			deletePendingJobsByTag(host, tt.tag)

			if exitCalled != tt.expectExit {
				t.Errorf("Expected exit=%v, got %v", tt.expectExit, exitCalled)
			}
			if tt.expectedOut != "" && out.String() != tt.expectedOut {
				t.Errorf("Expected output %q, got %q", tt.expectedOut, out.String())
			}
		})
	}
}

func TestDeletePendingJobsOlderThan(t *testing.T) {
	tests := []struct {
		name         string
		olderThan    string
		method       string
		path         string
		responseCode int
		responseBody string
		expectExit   bool
		expectOutput string
	}{
		{
			name:         "Success",
			olderThan:    "24h",
			method:       http.MethodDelete,
			path:         "/jobs/pending?older_than=24h",
			responseCode: http.StatusOK,
			responseBody: `{"deleted": 5}`,
			expectExit:   false,
			expectOutput: "Successfully deleted 5 pending jobs older than 24h.\n",
		},
		{
			name:         "Server Error",
			olderThan:    "24h",
			method:       http.MethodDelete,
			path:         "/jobs/pending?older_than=24h",
			responseCode: http.StatusInternalServerError,
			responseBody: "Internal Server Error",
			expectExit:   true,
			expectOutput: "Failed to delete pending jobs older than 24h: Internal Server Error\n",
		},
		{
			name:         "Decode Error",
			olderThan:    "24h",
			method:       http.MethodDelete,
			path:         "/jobs/pending?older_than=24h",
			responseCode: http.StatusOK,
			responseBody: `invalid json`,
			expectExit:   true,
			expectOutput: "Failed to decode response:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock Server
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method || r.URL.String() != tt.path {
					t.Errorf("Expected request %s %s, got %s %s", tt.method, tt.path, r.Method, r.URL.String())
				}
				w.WriteHeader(tt.responseCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer ts.Close()

			// Mock stdout
			oldStdout := stdout
			defer func() { stdout = oldStdout }()
			var buf bytes.Buffer
			stdout = &buf

			// Mock exitFunc
			oldExitFunc := exitFunc
			defer func() { exitFunc = oldExitFunc }()
			exitCalled := false
			exitFunc = func(code int) {
				exitCalled = true
			}

			deletePendingJobsOlderThan(ts.URL, tt.olderThan)

			if exitCalled != tt.expectExit {
				t.Errorf("Expected exitCalled=%v, got %v", tt.expectExit, exitCalled)
			}
			if !bytes.Contains(buf.Bytes(), []byte(tt.expectOutput)) {
				t.Errorf("Expected output to contain %q, got %q", tt.expectOutput, buf.String())
			}
		})
	}
}

func TestDeletePendingJobsByMatch(t *testing.T) {
	tests := []struct {
		name         string
		match        string
		method       string
		path         string
		statusCode   int
		responseBody string
		clientError  bool
		urlError     bool
		expectExit   bool
		expectedOut  string
	}{
		{
			name:         "Success",
			match:        "test-match",
			method:       http.MethodDelete,
			path:         "/jobs/pending",
			statusCode:   http.StatusOK,
			responseBody: `{"deleted": 3}`,
			expectExit:   false,
			expectedOut:  "Successfully deleted 3 pending jobs by match.\n",
		},
		{
			name:         "HTTP Error",
			match:        "test-match",
			method:       http.MethodDelete,
			path:         "/jobs/pending",
			statusCode:   http.StatusInternalServerError,
			responseBody: "internal server error",
			expectExit:   true,
			expectedOut:  "Failed to delete pending jobs by match: internal server error\n",
		},
		{
			name:         "JSON Decode Error",
			match:        "test-match",
			method:       http.MethodDelete,
			path:         "/jobs/pending",
			statusCode:   http.StatusOK,
			responseBody: `invalid json`,
			expectExit:   true,
		},
		{
			name:       "URL Error",
			match:      "test-match",
			urlError:   true,
			expectExit: true,
		},
		{
			name:        "Client Error",
			match:       "test-match",
			clientError: true,
			expectExit:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method {
					t.Errorf("Expected %s method, got %s", tt.method, r.Method)
				}
				if r.URL.Path != tt.path {
					t.Errorf("Expected %s path, got %s", tt.path, r.URL.Path)
				}
				if r.URL.Query().Get("match") != tt.match {
					t.Errorf("Expected match %s, got %s", tt.match, r.URL.Query().Get("match"))
				}
				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					fmt.Fprint(w, tt.responseBody)
				}
			}))
			defer ts.Close()

			var out bytes.Buffer
			oldStdout := stdout
			stdout = &out
			defer func() { stdout = oldStdout }()

			exitCalled := false
			oldExitFunc := exitFunc
			exitFunc = func(code int) { exitCalled = true }
			defer func() { exitFunc = oldExitFunc }()

			var host string
			if tt.urlError {
				host = "http://invalid\nurl"
			} else if tt.clientError {
				host = "http://localhost:0"
			} else {
				host = ts.URL
			}

			deletePendingJobsByMatch(host, tt.match)

			if exitCalled != tt.expectExit {
				t.Errorf("Expected exit=%v, got %v", tt.expectExit, exitCalled)
			}
			if tt.expectedOut != "" && out.String() != tt.expectedOut {
				t.Errorf("Expected output %q, got %q", tt.expectedOut, out.String())
			}
		})
	}
}
