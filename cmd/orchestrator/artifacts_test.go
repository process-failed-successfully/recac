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
}
