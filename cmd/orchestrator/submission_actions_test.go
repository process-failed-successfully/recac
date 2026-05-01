package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubmissionUpdateDependencies(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/dependencies", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		var req struct {
			DependsOn []string `json:"depends_on"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, []string{"DEP-1", "DEP-2"}, req.DependsOn)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Dependencies updated"}`))
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

	updateDependencies(server.URL, "JOB-1", []string{"DEP-1", "DEP-2"})

	assert.Equal(t, 0, exitCode)
}

func TestSubmissionUpdateDependencies_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateDependencies("http://invalid-host:12345", "JOB-1", []string{"DEP-1"})

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}

func TestSubmissionUpdatePriority(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/priority", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		var req struct {
			Priority int `json:"priority"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, 10, req.Priority)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Priority updated"}`))
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

	updatePriority(server.URL, "JOB-1", 10)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "priority updated to 10")
}

func TestSubmissionUpdatePriority_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updatePriority("http://invalid-host:12345", "JOB-1", 10)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}

func TestSubmissionUpdatePriority_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/priority", func(w http.ResponseWriter, r *http.Request) {
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

	updatePriority(server.URL, "JOB-1", 10)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to update priority")
}

func TestSubmissionUpdateAgent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/agent", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		var req struct {
			AgentProvider string `json:"agent_provider"`
			AgentModel    string `json:"agent_model"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "openai", req.AgentProvider)
		assert.Equal(t, "gpt-4", req.AgentModel)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Agent updated"}`))
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

	updateAgent(server.URL, "JOB-1", "openai", "gpt-4")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "agent updated to provider=openai model=gpt-4")
}

func TestSubmissionUpdateAgent_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateAgent("http://invalid-host:12345", "JOB-1", "openai", "gpt-4")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}

func TestSubmissionUpdateAgent_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/agent", func(w http.ResponseWriter, r *http.Request) {
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

	updateAgent(server.URL, "JOB-1", "openai", "gpt-4")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to update agent")
}

func TestSubmissionUpdateTimeout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/timeout", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		var req struct {
			Timeout string `json:"timeout"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "60s", req.Timeout)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Timeout updated"}`))
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

	updateTimeout(server.URL, "JOB-1", "60s")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "timeout updated to 60s")
}

func TestSubmissionUpdateTimeout_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateTimeout("http://invalid-host:12345", "JOB-1", "60s")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}

func TestSubmissionUpdateTimeout_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/timeout", func(w http.ResponseWriter, r *http.Request) {
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

	updateTimeout(server.URL, "JOB-1", "60s")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to update timeout")
}

func TestSubmissionSetJobOutput(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/output", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		var req struct {
			Outputs map[string]string `json:"outputs"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"foo": "bar"}, req.Outputs)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Output updated"}`))
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

	setJobOutput(server.URL, "JOB-1", "foo", "bar")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Successfully set output foo=bar for job JOB-1")
}

func TestSubmissionSetJobOutput_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	setJobOutput("http://invalid-host:12345", "JOB-1", "foo", "bar")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}

func TestSubmissionSetJobOutput_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/output", func(w http.ResponseWriter, r *http.Request) {
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

	setJobOutput(server.URL, "JOB-1", "foo", "bar")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to set job output")
}

func TestSubmissionSetJobProgress(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/progress", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		var req struct {
			Progress      *int    `json:"progress,omitempty"`
			StatusMessage *string `json:"status_message,omitempty"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.NotNil(t, req.Progress)
		assert.Equal(t, 50, *req.Progress)
		assert.NotNil(t, req.StatusMessage)
		assert.Equal(t, "Halfway there", *req.StatusMessage)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Progress updated"}`))
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

	progress := 50
	msg := "Halfway there"
	setJobProgress(server.URL, "JOB-1", &progress, &msg)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Successfully updated progress for job JOB-1")
}

func TestSubmissionSetJobProgress_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	progress := 50
	msg := "Halfway there"
	setJobProgress("http://invalid-host:12345", "JOB-1", &progress, &msg)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}

func TestSubmissionSetJobProgress_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/progress", func(w http.ResponseWriter, r *http.Request) {
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

	progress := 50
	msg := "Halfway there"
	setJobProgress(server.URL, "JOB-1", &progress, &msg)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to set job progress")
}

func TestSubmissionAddJobMetrics(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/metrics", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		var req struct {
			Metrics map[string]float64 `json:"metrics"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, map[string]float64{"cost": 1.23}, req.Metrics)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Metrics added"}`))
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

	addJobMetrics(server.URL, "JOB-1", "cost", 1.23)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Successfully added metric cost=1.23 for job JOB-1")
}

func TestSubmissionAddJobMetrics_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	addJobMetrics("http://invalid-host:12345", "JOB-1", "cost", 1.23)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}

func TestSubmissionAddJobMetrics_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/metrics", func(w http.ResponseWriter, r *http.Request) {
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

	addJobMetrics(server.URL, "JOB-1", "cost", 1.23)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to add job metrics")
}

func TestSubmissionHoldJobs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/hold", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "test-tag", r.URL.Query().Get("tag"))
		assert.Equal(t, "test-match", r.URL.Query().Get("match"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"held":2}`))
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

	holdJobs(server.URL, "test-match", "test-tag", "")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Successfully held 2 jobs")
}

func TestSubmissionHoldJobs_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	holdJobs("http://invalid-host:12345", "test-match", "test-tag", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}

func TestSubmissionHoldJobs_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/hold", func(w http.ResponseWriter, r *http.Request) {
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

	holdJobs(server.URL, "test-match", "test-tag", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to hold jobs")
}

func TestSubmissionUnholdJobs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/unhold", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "test-tag", r.URL.Query().Get("tag"))
		assert.Equal(t, "test-match", r.URL.Query().Get("match"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"unheld":2}`))
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

	unholdJobs(server.URL, "test-match", "test-tag", "")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Successfully unheld 2 jobs")
}

func TestSubmissionUnholdJobs_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	unholdJobs("http://invalid-host:12345", "test-match", "test-tag", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to connect to orchestrator")
}

func TestSubmissionUnholdJobs_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/unhold", func(w http.ResponseWriter, r *http.Request) {
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

	unholdJobs(server.URL, "test-match", "test-tag", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to unhold jobs")
}

func TestSubmissionUpdateDependencies_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1/dependencies", func(w http.ResponseWriter, r *http.Request) {
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

	updateDependencies(server.URL, "JOB-1", []string{"DEP-1"})

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to update dependencies")
}

func TestUpdateBulkPriority(t *testing.T) {
	tests := []struct {
		name        string
		match       string
		tag         string
		priority    int
		serverURL   string
		handler     http.HandlerFunc
		expectedOut string
		expectExit  bool
	}{
		{
			name:     "Success",
			match:    "test-match",
			tag:      "test-tag",
			priority: 5,
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/jobs/priority", r.URL.Path)
				assert.Equal(t, "PUT", r.Method)
				assert.Equal(t, "test-match", r.URL.Query().Get("match"))
				assert.Equal(t, "test-tag", r.URL.Query().Get("tag"))
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"updated": 3}`))
			},
			expectedOut: "Successfully updated priority for 3 jobs.",
		},
		{
			name:        "Connection Error",
			match:       "test-match",
			tag:         "test-tag",
			priority:    5,
			serverURL:   "http://localhost:0",
			expectedOut: "Failed to connect to orchestrator",
			expectExit:  true,
		},
		{
			name:     "Error Response",
			match:    "test-match",
			tag:      "test-tag",
			priority: 5,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`Internal Server Error`))
			},
			expectedOut: "Failed to update bulk priority",
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

			updateBulkPriority(serverURL, tt.match, tt.tag, tt.priority)

			assert.Equal(t, tt.expectExit, exitCalled)
			assert.Contains(t, buf.String(), tt.expectedOut)
		})
	}
}
