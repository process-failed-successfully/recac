package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMainApproveJob(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/approve", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Job approved"}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	approveJob(server.URL, "JOB-1")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Job JOB-1 approved successfully.")
}

func TestMainApproveJob_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	approveJob("http://invalid-host:12345", "JOB-1")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}

func TestMainApproveJob_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/approve", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`invalid request`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	approveJob(server.URL, "JOB-1")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to approve job")
}

func TestMainHoldJob(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/hold", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Job held"}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	holdJob(server.URL, "JOB-1")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Job JOB-1 held successfully.")
}

func TestMainHoldJob_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	holdJob("http://invalid-host:12345", "JOB-1")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}

func TestMainHoldJob_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/hold", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`invalid request`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	holdJob(server.URL, "JOB-1")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to hold job")
}

func TestMainUnholdJob(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/unhold", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Job unheld"}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	unholdJob(server.URL, "JOB-1")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Job JOB-1 unheld successfully.")
}

func TestMainUnholdJob_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	unholdJob("http://invalid-host:12345", "JOB-1")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}

func TestMainUnholdJob_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/unhold", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`invalid request`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	unholdJob(server.URL, "JOB-1")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to unhold job")
}

func TestApproveBulkJobs(t *testing.T) {
	tests := []struct {
		name        string
		match       string
		tag         string
		group       string
		serverURL   string
		handler     http.HandlerFunc
		expectedOut string
		expectExit  bool
	}{
		{
			name:  "Success",
			match: "test-match",
			tag:   "test-tag",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/jobs/approve", r.URL.Path)
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "test-match", r.URL.Query().Get("match"))
				assert.Equal(t, "test-tag", r.URL.Query().Get("tag"))
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"approved": 5}`))
			},
			expectedOut: "Successfully approved 5 jobs.",
		},
		{
			name:  "Success with Group",
			group: "test-group",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/jobs/approve", r.URL.Path)
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "test-group", r.URL.Query().Get("group"))
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"approved": 3}`))
			},
			expectedOut: "Successfully approved 3 jobs.",
		},
		{
			name:        "Connection Error",
			match:       "test-match",
			tag:         "test-tag",
			serverURL:   "http://localhost:0",
			expectedOut: "Failed to connect to orchestrator",
			expectExit:  true,
		},
		{
			name:  "Error Response",
			match: "test-match",
			tag:   "test-tag",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`Internal Server Error`))
			},
			expectedOut: "Failed to approve jobs",
			expectExit:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverURL := tt.serverURL
			if serverURL == "" && tt.handler != nil {
				server := httptest.NewServer(tt.handler)
				defer server.Close()
				serverURL = server.URL
			}

			var buf bytes.Buffer
			oldStdout := stdout
			stdout = &buf
			defer func() { stdout = oldStdout }()

			exitCalled := false
			oldExitFunc := exitFunc
			exitFunc = func(int) { exitCalled = true }
			defer func() { exitFunc = oldExitFunc }()

			approveBulkJobs(serverURL, tt.match, tt.tag, tt.group)

			assert.Equal(t, tt.expectExit, exitCalled)
			assert.Contains(t, buf.String(), tt.expectedOut)
		})
	}
}
