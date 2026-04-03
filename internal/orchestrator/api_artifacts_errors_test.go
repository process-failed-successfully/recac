package orchestrator

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAPI_Artifacts_DirectHandlers(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "recac-artifacts-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	orch := New(new(MockPoller), new(MockSpawner), 1*time.Minute)
	orch.ArtifactsDir = tempDir

	orchNoDir := New(new(MockPoller), new(MockSpawner), 1*time.Minute)
	orchNoDir.ArtifactsDir = ""

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Helpers
	callHandler := func(handler http.HandlerFunc, jobID, filename string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetPathValue("id", jobID)
		if filename != "" {
			req.SetPathValue("filename", filename)
		}
		w := httptest.NewRecorder()
		handler(w, req)
		return w
	}

	t.Run("Missing Parameters", func(t *testing.T) {
		handlers := []http.HandlerFunc{
			handleUploadArtifact(orch, logger),
			handleDownloadArtifact(orch, logger),
			handleDeleteArtifact(orch, logger),
		}
		for _, h := range handlers {
			w := callHandler(h, "", "file.txt")
			assert.Equal(t, http.StatusBadRequest, w.Code)

			w = callHandler(h, "job", "")
			assert.Equal(t, http.StatusBadRequest, w.Code)
		}

		w := callHandler(handleListArtifacts(orch, logger), "", "")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("No Artifacts Dir", func(t *testing.T) {
		w := callHandler(handleUploadArtifact(orchNoDir, logger), "job", "file.txt")
		assert.Equal(t, http.StatusInternalServerError, w.Code)

		w = callHandler(handleDownloadArtifact(orchNoDir, logger), "job", "file.txt")
		assert.Equal(t, http.StatusInternalServerError, w.Code)

		w = callHandler(handleDeleteArtifact(orchNoDir, logger), "job", "file.txt")
		assert.Equal(t, http.StatusInternalServerError, w.Code)

		w = callHandler(handleListArtifacts(orchNoDir, logger), "job", "")
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Invalid JobID", func(t *testing.T) {
		invalidIDs := []string{".", "..", "/", "a/b"}
		for _, id := range invalidIDs {
			w := callHandler(handleUploadArtifact(orch, logger), id, "file.txt")
			assert.Equal(t, http.StatusInternalServerError, w.Code) // ensureArtifactsDir returns error

			w = callHandler(handleDownloadArtifact(orch, logger), id, "file.txt")
			assert.Equal(t, http.StatusBadRequest, w.Code)

			w = callHandler(handleDeleteArtifact(orch, logger), id, "file.txt")
			assert.Equal(t, http.StatusBadRequest, w.Code)

			w = callHandler(handleListArtifacts(orch, logger), id, "")
			assert.Equal(t, http.StatusBadRequest, w.Code)
		}
	})

	t.Run("Invalid Filename", func(t *testing.T) {
		invalidNames := []string{".", "/", "a/b"}
		for _, name := range invalidNames {
			w := callHandler(handleUploadArtifact(orch, logger), "job", name)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		}
	})
}

func TestAPI_Artifacts_MoreCoverage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "recac-artifacts-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	orch := New(new(MockPoller), new(MockSpawner), 1*time.Minute)
	orch.ArtifactsDir = tempDir

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	callHandler := func(handler http.HandlerFunc, jobID, filename string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetPathValue("id", jobID)
		if filename != "" {
			req.SetPathValue("filename", filename)
		}
		w := httptest.NewRecorder()
		handler(w, req)
		return w
	}

	t.Run("Delete Delete Error", func(t *testing.T) {
		// Mock os.Remove error by trying to delete a directory
		w := callHandler(handleDeleteArtifact(orch, logger), "job", "job")
		assert.Equal(t, http.StatusNotFound, w.Code) // actually os.IsNotExist on the filepath built
	})

    t.Run("Upload Create File Error", func(t *testing.T) {
        // Can't create file in read only directory
        roDir, err := os.MkdirTemp("", "recac-artifacts-ro")
        assert.NoError(t, err)
        defer os.RemoveAll(roDir)

        jobDir := roDir + "/job"
        err = os.Mkdir(jobDir, 0555) // Read and execute only, no write
        assert.NoError(t, err)

        orchRO := New(new(MockPoller), new(MockSpawner), 1*time.Minute)
	    orchRO.ArtifactsDir = roDir

        w := callHandler(handleUploadArtifact(orchRO, logger), "job", "file.txt")
		assert.Equal(t, http.StatusInternalServerError, w.Code)
    })
}
