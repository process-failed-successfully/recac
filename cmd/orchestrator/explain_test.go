package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"recac/internal/agent"
	"recac/internal/orchestrator"
)

func TestExplainJob_Success(t *testing.T) {
	// 1. Mock the API Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/TEST-123" {
			job := orchestrator.JobInfo{
				ID:      "TEST-123",
				Summary: "Fix bug in login",
				Status:  "Failed",
				Error:   "Compilation failed: syntax error",
			}
			json.NewEncoder(w).Encode(job)
			return
		}

		if r.URL.Path == "/jobs/TEST-123/logs" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Building project...\nError: syntax error on line 42\nBuild failed."))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// 2. Mock the Agent Factory
	originalNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = originalNewAgentFunc }()

	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse("### Analysis\nThe job failed due to a syntax error on line 42. You should fix the missing semicolon.")

	newAgentFunc = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// 3. Capture stdout
	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	// Prevent exitFunc from actually exiting
	originalExitFunc := exitFunc
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = originalExitFunc }()

	// 4. Run the function
	explainJob(server.URL, "TEST-123", "mock-provider", "mock-model")

	// 5. Verify expectations
	assert.False(t, exitCalled, "explainJob should not exit on success")

	output := buf.String()
	assert.Contains(t, output, "Fetching job details for TEST-123...")
	assert.Contains(t, output, "Fetching logs for TEST-123...")
	assert.Contains(t, output, "Analyzing with AI...")

	// Check if the mock response is in the output
	// The output will be rendered by glamour, which adds ANSI escape codes, so we check for fragments
	assert.Contains(t, output, "Analysis")
	assert.Contains(t, output, "syntax error on line 42")
}

func TestExplainJob_JobNotFound(t *testing.T) {
	// 1. Mock the API Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Job not found"))
	}))
	defer server.Close()

	// 2. Capture stdout
	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	// Prevent exitFunc from actually exiting
	originalExitFunc := exitFunc
	exitCode := -1
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExitFunc }()

	// 3. Run the function
	explainJob(server.URL, "MISSING-123", "mock-provider", "mock-model")

	// 4. Verify expectations
	assert.Equal(t, 1, exitCode, "explainJob should exit with code 1 when job is not found")

	output := buf.String()
	assert.Contains(t, output, "Failed to fetch job details: Job not found")
}

func TestExplainJob_ConnectionError(t *testing.T) {
	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	originalExitFunc := exitFunc
	exitCode := -1
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = originalExitFunc }()

	explainJob("http://invalid-url:12345", "TEST-123", "", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
}

func TestExplainJob_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	originalExitFunc := exitFunc
	exitCode := -1
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = originalExitFunc }()

	explainJob(server.URL, "TEST-123", "", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to decode response:")
}

func TestExplainJob_LogsErrorAndTruncation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/TEST-123" {
			job := orchestrator.JobInfo{ID: "TEST-123"}
			json.NewEncoder(w).Encode(job)
			return
		}

		if r.URL.Path == "/jobs/TEST-123/logs" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	originalNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = originalNewAgentFunc }()

	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse("Mock explanation")
	newAgentFunc = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
		return mockAgent, nil
	}

	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	originalExitFunc := exitFunc
	exitCalled := false
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = originalExitFunc }()

	explainJob(server.URL, "TEST-123", "", "")

	assert.False(t, exitCalled)
	output := buf.String()
	assert.Contains(t, output, "Warning: Failed to fetch logs, status 500")
}

func TestExplainJob_LogsTruncation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/TEST-123" {
			job := orchestrator.JobInfo{ID: "TEST-123"}
			json.NewEncoder(w).Encode(job)
			return
		}

		if r.URL.Path == "/jobs/TEST-123/logs" {
			w.WriteHeader(http.StatusOK)
			// Create string with 1005 lines
			var logs string
			for i := 0; i < 1005; i++ {
				logs += "Log line\n"
			}
			w.Write([]byte(logs))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	originalNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = originalNewAgentFunc }()

	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse("Mock explanation")
	newAgentFunc = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
		return mockAgent, nil
	}

	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	originalExitFunc := exitFunc
	exitCalled := false
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = originalExitFunc }()

	explainJob(server.URL, "TEST-123", "", "")

	assert.False(t, exitCalled)
}

func TestExplainJob_AIFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/TEST-123" {
			job := orchestrator.JobInfo{ID: "TEST-123"}
			json.NewEncoder(w).Encode(job)
			return
		}

		if r.URL.Path == "/jobs/TEST-123/logs" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("logs"))
			return
		}
	}))
	defer server.Close()

	originalNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = originalNewAgentFunc }()

	mockAgent := agent.NewMockAgent()
	mockAgent.SetError(assert.AnError)
	newAgentFunc = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
		return mockAgent, nil
	}

	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	originalExitFunc := exitFunc
	exitCode := -1
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = originalExitFunc }()

	explainJob(server.URL, "TEST-123", "", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to get explanation from AI:")
}

func TestExplainJob_AIInitFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/TEST-123" {
			job := orchestrator.JobInfo{ID: "TEST-123"}
			json.NewEncoder(w).Encode(job)
			return
		}

		if r.URL.Path == "/jobs/TEST-123/logs" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("logs"))
			return
		}
	}))
	defer server.Close()

	originalNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = originalNewAgentFunc }()

	newAgentFunc = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
		return nil, assert.AnError
	}

	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	originalExitFunc := exitFunc
	exitCode := -1
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = originalExitFunc }()

	explainJob(server.URL, "TEST-123", "", "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to initialize AI agent:")
}
