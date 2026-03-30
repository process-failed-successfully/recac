package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExportJobs_JSON(t *testing.T) {
	// Mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/export", r.URL.Path)
		assert.Equal(t, "json", r.URL.Query().Get("format"))

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `[{"id":"job-1","summary":"A job"}]`)
	}))
	defer ts.Close()

	// Capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Temp file for export
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "export.json")

	// Call exportJobs
	exportJobs(ts.URL, outFile, "json")

	// Assertions
	assert.Contains(t, buf.String(), "Jobs successfully exported")
	assert.FileExists(t, outFile)

	content, err := os.ReadFile(outFile)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "job-1")
}

func TestExportJobs_Stdout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `[{"id":"job-2"}]`)
	}))
	defer ts.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exportJobs(ts.URL, "-", "json")

	assert.Contains(t, buf.String(), "job-2")
	assert.NotContains(t, buf.String(), "successfully exported")
}

func TestExportJobs_InvalidFormat(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Capture exit
	exited := false
	oldExit := exitFunc
	exitFunc = func(code int) { exited = true }
	defer func() { exitFunc = oldExit }()

	exportJobs("http://localhost:2112", "out.txt", "xml")

	assert.True(t, exited)
	assert.Contains(t, buf.String(), "Invalid format")
}

func TestExportJobs_Errors(t *testing.T) {
	originalExit := exitFunc
	defer func() { exitFunc = originalExit }()

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}

	originalStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = originalStdout }()

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		exportJobs("http://invalid-host", "out.json", "json")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("BadStatusCode", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		exportJobs(server.URL, "out.json", "json")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to export jobs")
	})

	t.Run("LocalFileCreationFailure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"id":"job-1"}]`))
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()

		// Use a path that is guaranteed to fail creation (e.g., directory that doesn't exist)
		badPath := filepath.Join("non-existent-dir", "out.json")
		exportJobs(server.URL, badPath, "json")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to create output file")
	})
}

func TestExportPipeline(t *testing.T) {
	originalExit := exitFunc
	defer func() { exitFunc = originalExit }()

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}

	originalStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = originalStdout }()

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		exportPipeline("http://invalid-host", "out.yaml")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("BadStatusCode", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		exportPipeline(server.URL, "out.yaml")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to export pipeline")
	})

	t.Run("Stdout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/jobs/export/pipeline", r.URL.Path)
			assert.Equal(t, "exported-pipeline", r.URL.Query().Get("name"))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`name: my-pipeline`))
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		exportPipeline(server.URL, "-")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "name: my-pipeline")
	})

	t.Run("FileSuccess", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/jobs/export/pipeline", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`name: my-pipeline`))
		}))
		defer server.Close()

		tmpDir := t.TempDir()
		outFile := filepath.Join(tmpDir, "pipeline.yaml")

		exitCode = 0
		buf.Reset()
		exportPipeline(server.URL, outFile)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "Pipeline successfully exported")

		content, err := os.ReadFile(outFile)
		assert.NoError(t, err)
		assert.Equal(t, "name: my-pipeline", string(content))
	})

	t.Run("FileCreationError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`name: my-pipeline`))
		}))
		defer server.Close()

		badPath := filepath.Join("non-existent-dir", "pipeline.yaml")

		exitCode = 0
		buf.Reset()
		exportPipeline(server.URL, badPath)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to create output file")
	})
}

func TestExportGraph(t *testing.T) {
	originalExit := exitFunc
	defer func() { exitFunc = originalExit }()

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}

	originalStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = originalStdout }()

	t.Run("InvalidFormat", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		exportGraph("http://localhost", "out.dot", "xml")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Invalid format: xml")
	})

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		exportGraph("http://invalid-host", "out.dot", "dot")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("BadStatusCode", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		exportGraph(server.URL, "out.dot", "dot")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to export graph")
	})

	t.Run("Stdout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/jobs/export/graph", r.URL.Path)
			assert.Equal(t, "mermaid", r.URL.Query().Get("format"))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`graph TD; A-->B;`))
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		exportGraph(server.URL, "-", "mermaid")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "graph TD; A-->B;")
	})

	t.Run("StdoutSuccessPlantUML", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/jobs/export/graph", r.URL.Path)
			assert.Equal(t, "plantuml", r.URL.Query().Get("format"))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("@startuml A-->B; @enduml"))
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		exportGraph(server.URL, "-", "plantuml")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "@startuml A-->B; @enduml")
	})

	t.Run("FileSuccess", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/jobs/export/graph", r.URL.Path)
			assert.Equal(t, "dot", r.URL.Query().Get("format"))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`digraph G { A -> B; }`))
		}))
		defer server.Close()

		tmpDir := t.TempDir()
		outFile := filepath.Join(tmpDir, "graph.dot")

		exitCode = 0
		buf.Reset()
		exportGraph(server.URL, outFile, "dot")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "Graph successfully exported")

		content, err := os.ReadFile(outFile)
		assert.NoError(t, err)
		assert.Equal(t, "digraph G { A -> B; }", string(content))
	})

	t.Run("FileCreationError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`digraph G { A -> B; }`))
		}))
		defer server.Close()

		badPath := filepath.Join("non-existent-dir", "graph.dot")

		exitCode = 0
		buf.Reset()
		exportGraph(server.URL, badPath, "dot")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to create output file")
	})
}
