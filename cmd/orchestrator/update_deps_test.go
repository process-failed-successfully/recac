package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateDependencies_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-123/dependencies", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)

		var reqBody struct {
			DependsOn []string `json:"depends_on"`
		}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		assert.NoError(t, err)
		assert.Equal(t, []string{"DEP-1", "DEP-2"}, reqBody.DependsOn)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"depends_on": ["DEP-1", "DEP-2"]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateDependencies(server.URL, "JOB-123", []string{"DEP-1", "DEP-2"})

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Job JOB-123 dependencies updated to: DEP-1, DEP-2")
	assert.Equal(t, 0, exitCode)
}

func TestUpdateDependencies_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-123/dependencies", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`invalid dependencies`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateDependencies(server.URL, "JOB-123", []string{"DEP-1"})

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to update dependencies: invalid dependencies")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkDependencies_ConnectionError(t *testing.T) {
	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkDependencies("http://[::1]:namedport", "", "tag-a", []string{"DEP-1"})
	updateBulkDependencies("http://localhost:12345", "", "tag-a", []string{"DEP-1"})

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to parse URL:")
	assert.Contains(t, out, "Failed to connect to orchestrator at")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkTimeout_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/timeout", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)

		var reqBody struct {
			Timeout string `json:"timeout"`
		}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		assert.NoError(t, err)
		assert.Equal(t, "120", reqBody.Timeout)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"updated": 3}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkTimeout(server.URL, "match-a", "", "120")

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Successfully updated timeouts for 3 pending jobs.")
	assert.Equal(t, 0, exitCode)
}

func TestUpdateBulkTimeout_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/timeout", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`invalid timeout`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkTimeout(server.URL, "", "tag-a", "120")

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to update bulk timeouts: invalid timeout")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkTimeout_ConnectionError(t *testing.T) {
	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkTimeout("http://[::1]:namedport", "", "tag-a", "100")
	updateBulkTimeout("http://localhost:12345", "", "tag-a", "100")

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to parse URL:")
	assert.Contains(t, out, "Failed to connect to orchestrator at")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkTimeout_InvalidJSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/timeout", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkTimeout(server.URL, "", "tag-a", "100")

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to decode response:")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkTags_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/tags", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"updated": 3}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	updateBulkTags(server.URL, "match-a", "", []string{"tag1", "tag2"})

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Successfully updated tags for 3 pending jobs to: tag1, tag2")
}

func TestUpdateBulkTags_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/tags", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`error`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkTags(server.URL, "match-a", "", []string{"tag1"})

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to update bulk tags: error")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkTags_ConnectionError(t *testing.T) {
	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkTags("http://[::1]:namedport", "match-a", "", []string{"tag1"})
	updateBulkTags("http://localhost:12345", "match-a", "", []string{"tag1"})

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to parse URL")
	assert.Contains(t, out, "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkTags_InvalidJSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/tags", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkTags(server.URL, "match-a", "", []string{"tag1"})

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to decode response:")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkEnvVars_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/env", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"updated": 3}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	updateBulkEnvVars(server.URL, "match-a", "", map[string]string{"K": "V"})

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Successfully updated environment variables for 3 pending jobs.")
}

func TestUpdateBulkEnvVars_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/env", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`error`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkEnvVars(server.URL, "match-a", "", map[string]string{"K": "V"})

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to update bulk env vars: error")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkEnvVars_ConnectionError(t *testing.T) {
	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkEnvVars("http://[::1]:namedport", "match-a", "", map[string]string{"K": "V"})
	updateBulkEnvVars("http://localhost:12345", "match-a", "", map[string]string{"K": "V"})

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to parse URL")
	assert.Contains(t, out, "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkEnvVars_InvalidJSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/env", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkEnvVars(server.URL, "match-a", "", map[string]string{"K": "V"})

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to decode response:")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkAgent_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/agent", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"updated": 3}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	updateBulkAgent(server.URL, "match-a", "", "provider", "model")

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Successfully updated agents for 3 pending jobs.")
}

func TestUpdateBulkAgent_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/agent", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`error`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkAgent(server.URL, "match-a", "", "provider", "model")

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to update bulk agents: error")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkAgent_ConnectionError(t *testing.T) {
	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkAgent("http://[::1]:namedport", "match-a", "", "provider", "model")
	updateBulkAgent("http://localhost:12345", "match-a", "", "provider", "model")

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to parse URL")
	assert.Contains(t, out, "Failed to connect to orchestrator")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkAgent_InvalidJSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/agent", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkAgent(server.URL, "match-a", "", "provider", "model")

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to decode response:")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkDependencies_InvalidJSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/dependencies", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkDependencies(server.URL, "", "tag-a", []string{"DEP-1"})

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to decode response:")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkDependencies_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/dependencies", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)

		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")
		assert.True(t, tag == "tag-a" || match == "match-a")

		var reqBody struct {
			DependsOn []string `json:"depends_on"`
		}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		assert.NoError(t, err)
		assert.Equal(t, []string{"DEP-1", "DEP-2"}, reqBody.DependsOn)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"updated": 3}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	// Test Tag
	updateBulkDependencies(server.URL, "", "tag-a", []string{"DEP-1", "DEP-2"})

	// Test Match
	updateBulkDependencies(server.URL, "match-a", "", []string{"DEP-1", "DEP-2"})

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Successfully updated dependencies for 3 jobs to: DEP-1, DEP-2")
	assert.Equal(t, 0, exitCode)
}

func TestUpdateBulkDependencies_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/dependencies", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`invalid dependencies`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateBulkDependencies(server.URL, "", "tag-a", []string{"DEP-1"})

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to update dependencies: invalid dependencies")
	assert.Equal(t, 1, exitCode)
}
