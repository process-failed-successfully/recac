package orchestrator

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAPI_Artifacts(t *testing.T) {
	// Create a temporary directory for artifacts
	tempDir, err := os.MkdirTemp("", "recac-artifacts-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	orch := New(mockPoller, mockSpawner, 1*time.Minute)
	orch.ArtifactsDir = tempDir

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, nil)

	server := httptest.NewServer(mux)
	defer server.Close()

	client := server.Client()

	jobID := "TEST-JOB"
	filename := "test.txt"
	fileContent := []byte("hello artifact world")

	// 1. Upload Artifact
	t.Run("Upload Artifact", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPut, server.URL+"/jobs/"+jobID+"/artifacts/"+filename, bytes.NewReader(fileContent))
		assert.NoError(t, err)

		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify file exists on disk
		content, err := os.ReadFile(filepath.Join(tempDir, jobID, filename))
		assert.NoError(t, err)
		assert.Equal(t, fileContent, content)
	})

	// 2. List Artifacts
	t.Run("List Artifacts", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/jobs/"+jobID+"/artifacts", nil)
		assert.NoError(t, err)

		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string][]string
		err = json.NewDecoder(resp.Body).Decode(&result)
		assert.NoError(t, err)

		artifacts, ok := result["artifacts"]
		assert.True(t, ok)
		assert.Contains(t, artifacts, filename)
	})

	// 3. Download Artifact
	t.Run("Download Artifact", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/jobs/"+jobID+"/artifacts/"+filename, nil)
		assert.NoError(t, err)

		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		content, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, fileContent, content)
	})

	// 4. Delete Artifact
	t.Run("Delete Artifact", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, server.URL+"/jobs/"+jobID+"/artifacts/"+filename, nil)
		assert.NoError(t, err)

		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify file is gone
		_, err = os.Stat(filepath.Join(tempDir, jobID, filename))
		assert.True(t, os.IsNotExist(err))
	})

	// 5. List Empty Artifacts
	t.Run("List Empty Artifacts", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/jobs/EMPTY-JOB/artifacts", nil)
		assert.NoError(t, err)

		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string][]string
		err = json.NewDecoder(resp.Body).Decode(&result)
		assert.NoError(t, err)

		artifacts, ok := result["artifacts"]
		assert.True(t, ok)
		assert.Empty(t, artifacts)
	})

    // 6. Download Not Found
	t.Run("Download Not Found", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/jobs/"+jobID+"/artifacts/missing.txt", nil)
		assert.NoError(t, err)

		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	// 7. Path Traversal in jobID
	t.Run("Path Traversal in JobID", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "/jobs/../artifacts/"+filename, nil)
		assert.NoError(t, err)
		req.SetPathValue("id", "..")
		req.SetPathValue("filename", filename)

		rr := httptest.NewRecorder()
		handleDownloadArtifact(orch, logger).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	// 8. Path Traversal in filename
	t.Run("Path Traversal in Filename", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "/jobs/"+jobID+"/artifacts/..", nil)
		assert.NoError(t, err)
		req.SetPathValue("id", jobID)
		req.SetPathValue("filename", "..")

		rr := httptest.NewRecorder()
		handleDownloadArtifact(orch, logger).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	// 9. Path Traversal in filename with slashes
	t.Run("Path Traversal in Filename with Slashes", func(t *testing.T) {
		// NewRequest with URL parsing treats %2F as /, so filepath.Base will act on it.
		// net/http client may unescape and normalize. We will just use the handler directly if needed,
		// or pass it in such a way it stays URL encoded, but http.ServeMux handles standard paths.
		// For coverage, we simulate by directly invoking the handler.
		req, err := http.NewRequest(http.MethodGet, "/jobs/"+jobID+"/artifacts/..%2F..%2Fetc%2Fpasswd", nil)
		assert.NoError(t, err)

		// Setup req path values as if parsed by mux
		req.SetPathValue("id", jobID)
		req.SetPathValue("filename", "../../etc/passwd")

		rr := httptest.NewRecorder()
		handleDownloadArtifact(orch, logger).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}
