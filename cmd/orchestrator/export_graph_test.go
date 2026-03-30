package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExportGraph_SuccessStdout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/export/graph", r.URL.Path)
		assert.Equal(t, "mermaid", r.URL.Query().Get("format"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("graph TD;"))
	}))
	defer server.Close()

	var out bytes.Buffer
	stdout = &out
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}

	exportGraph(server.URL, "-", "mermaid")

	assert.False(t, exitCalled)
	assert.Equal(t, "graph TD;", out.String())
}

func TestExportGraph_SuccessStdoutPlantUML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/export/graph", r.URL.Path)
		assert.Equal(t, "plantuml", r.URL.Query().Get("format"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("@startuml"))
	}))
	defer server.Close()

	var out bytes.Buffer
	stdout = &out

	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}

	exportGraph(server.URL, "-", "plantuml")

	assert.False(t, exitCalled)
	assert.Equal(t, "@startuml", out.String())
}

func TestExportGraph_SuccessFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/export/graph", r.URL.Path)
		assert.Equal(t, "dot", r.URL.Query().Get("format"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("digraph G {"))
	}))
	defer server.Close()

	tmpFile, _ := os.CreateTemp("", "graph_*.dot")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	var out bytes.Buffer
	stdout = &out
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}

	exportGraph(server.URL, tmpFile.Name(), "dot")

	assert.False(t, exitCalled)
	assert.Contains(t, out.String(), "successfully exported")

	content, _ := os.ReadFile(tmpFile.Name())
	assert.Equal(t, "digraph G {", string(content))
}

func TestExportGraph_InvalidFormat(t *testing.T) {
	var out bytes.Buffer
	stdout = &out
	exitCalled := false
	exitCode := 0
	exitFunc = func(code int) {
		exitCalled = true
		exitCode = code
	}

	exportGraph("http://localhost", "-", "unknown")

	assert.True(t, exitCalled)
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Invalid format: unknown")
}

func TestExportGraph_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	var out bytes.Buffer
	stdout = &out
	exitCalled := false
	exitCode := 0
	exitFunc = func(code int) {
		exitCalled = true
		exitCode = code
	}

	exportGraph(server.URL, "-", "mermaid")

	assert.True(t, exitCalled)
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to export graph: Internal Server Error")
}
