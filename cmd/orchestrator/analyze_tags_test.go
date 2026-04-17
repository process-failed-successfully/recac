package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnalyzeTags_JSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/analyze/tags", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"tags":[{"tag":"test","total_jobs":10,"successful_jobs":8,"failed_jobs":2,"success_rate":0.8,"average_duration":12000000000,"average_cost":0.01,"total_cost":0.1}]}`))
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

	analyzeTags(server.URL, 10, "json")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), `"tag": "test"`)
}

func TestAnalyzeTags_Text(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/analyze/tags", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"tags":[{"tag":"test","total_jobs":10,"successful_jobs":8,"failed_jobs":2,"success_rate":0.8,"average_duration":12000000000,"average_cost":0.01,"total_cost":0.1}]}`))
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

	analyzeTags(server.URL, 10, "")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "TAG PERFORMANCE ANALYSIS")
	assert.Contains(t, out.String(), "test")
}

func TestAnalyzeTags_Text_Empty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/analyze/tags", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"tags":[]}`))
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

	analyzeTags(server.URL, 10, "")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "TAG PERFORMANCE ANALYSIS")
	assert.Contains(t, out.String(), "No tag data available.")
}

func TestAnalyzeTags_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/analyze/tags", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`internal server error`))
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

	analyzeTags(server.URL, 10, "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Error from server")
}

func TestAnalyzeTags_ConnectionError(t *testing.T) {
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	analyzeTags("http://localhost:12345", 10, "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Error fetching tag analysis")
}

func TestAnalyzeTags_DecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/analyze/tags", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid-json}`))
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

	analyzeTags(server.URL, 10, "")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Error decoding response")
}
