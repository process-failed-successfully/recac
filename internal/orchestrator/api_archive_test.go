package orchestrator

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAPI_ArchiveJob(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)

	orch := New(mockPoller, mockSpawner, 1*time.Minute)

	// Add an existing job to history
	jobID := "archive-123"
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:      jobID,
		Status:  "Completed",
		Summary: "Test Archive Job",
	})

	// Mock GetLogs
	mockLogs := io.NopCloser(bytes.NewBufferString("Line 1\nLine 2\n"))
	mockSpawner.On("GetLogs", mock.Anything, jobID).Return(mockLogs, nil).Once()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Run("Success", func(t *testing.T) {
		url := server.URL + "/jobs/" + jobID + "/archive"
		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/gzip", resp.Header.Get("Content-Type"))
		assert.Equal(t, "attachment; filename=\"archive-123.tar.gz\"", resp.Header.Get("Content-Disposition"))

		// Read and un-tar the response
		gzReader, err := gzip.NewReader(resp.Body)
		require.NoError(t, err)
		defer gzReader.Close()

		tarReader := tar.NewReader(gzReader)

		var filesFound []string
		var logsContent string

		for {
			hdr, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			filesFound = append(filesFound, hdr.Name)

			if hdr.Name == "logs.txt" {
				b, err := io.ReadAll(tarReader)
				require.NoError(t, err)
				logsContent = string(b)
			}
		}

		assert.Contains(t, filesFound, "job.json")
		assert.Contains(t, filesFound, "logs.txt")
		assert.Equal(t, "Line 1\nLine 2\n", logsContent)
	})

	t.Run("Job Not Found", func(t *testing.T) {
		url := server.URL + "/jobs/non-existent/archive"
		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
