package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArtifactsCLI(t *testing.T) {
	jobID := "CLI-JOB"
	filename := "test_artifact.txt"
	fileContent := []byte("cli artifact test")

	// Set up a mock server
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /jobs/{id}/artifacts/{filename}", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, jobID, r.PathValue("id"))
		assert.Equal(t, filename, r.PathValue("filename"))
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Uploaded")
	})
	mux.HandleFunc("GET /jobs/{id}/artifacts/{filename}", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, jobID, r.PathValue("id"))
		assert.Equal(t, filename, r.PathValue("filename"))
		w.WriteHeader(http.StatusOK)
		w.Write(fileContent)
	})
	mux.HandleFunc("GET /jobs/{id}/artifacts", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, jobID, r.PathValue("id"))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string][]string{"artifacts": {filename}})
	})
	mux.HandleFunc("DELETE /jobs/{id}/artifacts/{filename}", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, jobID, r.PathValue("id"))
		assert.Equal(t, filename, r.PathValue("filename"))
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Deleted")
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	errorMux := http.NewServeMux()
	errorMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Internal Server Error")
	})
	errorServer := httptest.NewServer(errorMux)
	defer errorServer.Close()

	// Redirect stdout to buffer
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Provide a dummy exit func that just returns so we can test the output
	oldExit := exitFunc
	exitFunc = func(code int) {
		if code != 0 {
			panic(fmt.Sprintf("exit %d", code))
		}
	}
	defer func() { exitFunc = oldExit }()

	t.Run("uploadArtifact", func(t *testing.T) {
		buf.Reset()
		tempFile := filepath.Join(t.TempDir(), filename)
		err := os.WriteFile(tempFile, fileContent, 0644)
		assert.NoError(t, err)

		uploadArtifact(server.URL, jobID, tempFile)
		assert.Contains(t, buf.String(), "Successfully uploaded artifact")
	})

	t.Run("downloadArtifact", func(t *testing.T) {
		buf.Reset()
		outDir := t.TempDir()
		outPath := filepath.Join(outDir, "downloaded.txt")

		downloadArtifact(server.URL, jobID, filename, outPath)
		assert.Contains(t, buf.String(), "Successfully downloaded artifact")

		content, err := os.ReadFile(outPath)
		assert.NoError(t, err)
		assert.Equal(t, fileContent, content)
	})

	t.Run("listArtifacts", func(t *testing.T) {
		buf.Reset()
		listArtifacts(server.URL, jobID)
		assert.Contains(t, buf.String(), "Artifacts for job")
		assert.Contains(t, buf.String(), filename)
	})

	t.Run("deleteArtifact", func(t *testing.T) {
		buf.Reset()
		deleteArtifact(server.URL, jobID, filename)
		assert.Contains(t, buf.String(), "Successfully deleted artifact")
	})

	t.Run("uploadArtifact_FileOpenError", func(t *testing.T) {
		buf.Reset()
		assert.PanicsWithValue(t, "exit 1", func() {
			uploadArtifact(server.URL, jobID, "nonexistent_file.txt")
		})
		assert.Contains(t, buf.String(), "Failed to open file")
	})

	t.Run("uploadArtifact_ConnectionError", func(t *testing.T) {
		buf.Reset()
		tempFile := filepath.Join(t.TempDir(), filename)
		os.WriteFile(tempFile, fileContent, 0644)
		assert.PanicsWithValue(t, "exit 1", func() {
			uploadArtifact("http://localhost:0", jobID, tempFile)
		})
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("uploadArtifact_HTTPError", func(t *testing.T) {
		buf.Reset()
		tempFile := filepath.Join(t.TempDir(), filename)
		os.WriteFile(tempFile, fileContent, 0644)
		assert.PanicsWithValue(t, "exit 1", func() {
			uploadArtifact(errorServer.URL, jobID, tempFile)
		})
		assert.Contains(t, buf.String(), "Failed to upload artifact")
	})

	t.Run("downloadArtifact_ConnectionError", func(t *testing.T) {
		buf.Reset()
		assert.PanicsWithValue(t, "exit 1", func() {
			downloadArtifact("http://localhost:0", jobID, filename, "")
		})
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("downloadArtifact_HTTPError", func(t *testing.T) {
		buf.Reset()
		assert.PanicsWithValue(t, "exit 1", func() {
			downloadArtifact(errorServer.URL, jobID, filename, "")
		})
		assert.Contains(t, buf.String(), "Failed to download artifact")
	})

	t.Run("downloadArtifact_FileError", func(t *testing.T) {
		buf.Reset()
		assert.PanicsWithValue(t, "exit 1", func() {
			downloadArtifact(server.URL, jobID, filename, "/invalid/path/file.txt")
		})
		assert.Contains(t, buf.String(), "Failed to create output file")
	})

	t.Run("listArtifacts_ConnectionError", func(t *testing.T) {
		buf.Reset()
		assert.PanicsWithValue(t, "exit 1", func() {
			listArtifacts("http://localhost:0", jobID)
		})
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("listArtifacts_HTTPError", func(t *testing.T) {
		buf.Reset()
		assert.PanicsWithValue(t, "exit 1", func() {
			listArtifacts(errorServer.URL, jobID)
		})
		assert.Contains(t, buf.String(), "Failed to list artifacts")
	})

	t.Run("deleteArtifact_ConnectionError", func(t *testing.T) {
		buf.Reset()
		assert.PanicsWithValue(t, "exit 1", func() {
			deleteArtifact("http://localhost:0", jobID, filename)
		})
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("deleteArtifact_HTTPError", func(t *testing.T) {
		buf.Reset()
		assert.PanicsWithValue(t, "exit 1", func() {
			deleteArtifact(errorServer.URL, jobID, filename)
		})
		assert.Contains(t, buf.String(), "Failed to delete artifact")
	})
}

func TestListArtifacts_Empty(t *testing.T) {
	jobID := "CLI-JOB-EMPTY"

	mux := http.NewServeMux()
	mux.HandleFunc("GET /jobs/{id}/artifacts", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, jobID, r.PathValue("id"))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string][]string{"artifacts": {}})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitFunc = func(code int) {
		if code != 0 {
			panic(fmt.Sprintf("exit %d", code))
		}
	}
	defer func() { exitFunc = oldExit }()

	listArtifacts(server.URL, jobID)
	assert.Contains(t, buf.String(), "No artifacts found for job")
}

func TestListArtifacts_InvalidJSON(t *testing.T) {
	jobID := "CLI-JOB-INVALID"

	mux := http.NewServeMux()
	mux.HandleFunc("GET /jobs/{id}/artifacts", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, jobID, r.PathValue("id"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitFunc = func(code int) {
		if code != 0 {
			panic(fmt.Sprintf("exit %d", code))
		}
	}
	defer func() { exitFunc = oldExit }()

	assert.PanicsWithValue(t, "exit 1", func() {
		listArtifacts(server.URL, jobID)
	})
	assert.Contains(t, buf.String(), "Failed to decode response")
}

func TestDownloadArtifact_FallbackOutPath(t *testing.T) {
	jobID := "CLI-JOB"
	filename := "test_artifact_fallback.txt"
	fileContent := []byte("cli artifact fallback test")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /jobs/{id}/artifacts/{filename}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(fileContent)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitFunc = func(code int) {
		if code != 0 {
			panic(fmt.Sprintf("exit %d", code))
		}
	}
	defer func() { exitFunc = oldExit }()

	// Save original working dir
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)

	// Change to a temp dir so we don't write to the project root
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)

	downloadArtifact(server.URL, jobID, filename, "")
	assert.Contains(t, buf.String(), "Successfully downloaded artifact")

	content, err := os.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, fileContent, content)
}

func TestUploadArtifact_RequestCreationError(t *testing.T) {
    // This is hard to simulate without mocking http.NewRequest directly,
    // but we can try to pass an invalid URL.
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitFunc = func(code int) {
		if code != 0 {
			panic(fmt.Sprintf("exit %d", code))
		}
	}
	defer func() { exitFunc = oldExit }()

    tempFile := filepath.Join(t.TempDir(), "test.txt")
    os.WriteFile(tempFile, []byte("test"), 0644)

    // Using a control character in the URL to trigger http.NewRequest error
	assert.PanicsWithValue(t, "exit 1", func() {
		uploadArtifact("http://localhost:8080\x00invalid", "job1", tempFile)
	})
	assert.Contains(t, buf.String(), "Failed to create request")
}

func TestDeleteArtifact_RequestCreationError(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitFunc = func(code int) {
		if code != 0 {
			panic(fmt.Sprintf("exit %d", code))
		}
	}
	defer func() { exitFunc = oldExit }()

	assert.PanicsWithValue(t, "exit 1", func() {
		deleteArtifact("http://localhost:8080\x00invalid", "job1", "file.txt")
	})
	assert.Contains(t, buf.String(), "Failed to create request")
}

func TestDownloadArtifact_ResponseBodyError(t *testing.T) {
	jobID := "CLI-JOB"
	filename := "test_artifact_error.txt"

	mux := http.NewServeMux()
	mux.HandleFunc("GET /jobs/{id}/artifacts/{filename}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10")
		w.WriteHeader(http.StatusOK)
		// Close connection immediately to cause io.Copy to fail
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitFunc = func(code int) {
		if code != 0 {
			panic(fmt.Sprintf("exit %d", code))
		}
	}
	defer func() { exitFunc = oldExit }()

	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "downloaded_error.txt")

	assert.PanicsWithValue(t, "exit 1", func() {
		downloadArtifact(server.URL, jobID, filename, outPath)
	})
	assert.Contains(t, buf.String(), "Failed to save artifact")
}
