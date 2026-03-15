package orchestrator

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestBulkArchiveAPI(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))


	// Setup mock poller and spawner
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	mockSpawner.On("GetLogs", mock.Anything, "JOB-ACTIVE").Return(io.NopCloser(strings.NewReader("Active Logs")), nil)
	mockSpawner.On("GetLogs", mock.Anything, "JOB-COMPLETED").Return(io.NopCloser(strings.NewReader("Completed Logs")), nil)
	mockSpawner.On("GetLogs", mock.Anything, "JOB-IGNORED").Return(io.NopCloser(strings.NewReader("Ignored Logs")), nil)


	orch := New(mockPoller, mockSpawner, 1*time.Second)

	// Inject active and completed jobs
	activeJob := JobInfo{
		ID:        "JOB-ACTIVE",
		Status:    "Running",
		Summary:   "Active Test",
		StartTime: time.Now(),
		WorkItem: WorkItem{
			ID:      "JOB-ACTIVE",
			Summary: "Active Test",
			Tags:    []string{"bulk-test", "another-tag"},
		},
	}
	orch.mu.Lock()
orch.activeJobs["JOB-ACTIVE"] = activeJob
orch.mu.Unlock()

	completedJob := JobInfo{
		ID:        "JOB-COMPLETED",
		Status:    "Completed",
		Summary:   "Completed Test",
		StartTime: time.Now(),
		EndTime:   time.Now(),
		WorkItem: WorkItem{
			ID:      "JOB-COMPLETED",
			Summary: "Completed Test",
			Tags:    []string{"bulk-test"},
		},
	}
	orch.completedJobs = []JobInfo{completedJob}

	ignoredJob := JobInfo{
		ID:        "JOB-IGNORED",
		Status:    "Failed",
		Summary:   "Ignored Test",
		StartTime: time.Now(),
		EndTime:   time.Now(),
		WorkItem: WorkItem{
			ID:      "JOB-IGNORED",
			Summary: "Ignored Test",
			Tags:    []string{"other-tag"},
		},
	}
	orch.completedJobs = append(orch.completedJobs, ignoredJob)



	// Setup HTTP server
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Run("Valid Tag", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/jobs/archive/bulk?tag=bulk-test", server.URL))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "application/gzip", resp.Header.Get("Content-Type"))
		require.Equal(t, `attachment; filename="bulk_archive.tar.gz"`, resp.Header.Get("Content-Disposition"))

		// Read tar.gz
		gzReader, err := gzip.NewReader(resp.Body)
		require.NoError(t, err)
		defer gzReader.Close()

		tarReader := tar.NewReader(gzReader)

		foundFiles := make(map[string]bool)
		for {
			hdr, err := tarReader.Next()
			if err == io.EOF {
				break // End of archive
			}
			require.NoError(t, err)

			fileData, err := io.ReadAll(tarReader)
			require.NoError(t, err)
			foundFiles[hdr.Name] = true

			if strings.HasSuffix(hdr.Name, "job.json") {
				var parsedJob JobInfo
				err = json.Unmarshal(fileData, &parsedJob)
				require.NoError(t, err)
				require.True(t, parsedJob.ID == "JOB-ACTIVE" || parsedJob.ID == "JOB-COMPLETED")
			} else if strings.HasSuffix(hdr.Name, "logs.txt") {
				if strings.Contains(hdr.Name, "JOB-ACTIVE") {
					require.Equal(t, "Active Logs", string(fileData))
				} else if strings.Contains(hdr.Name, "JOB-COMPLETED") {
					require.Equal(t, "Completed Logs", string(fileData))
				}
			}
		}

		require.True(t, foundFiles["JOB-ACTIVE/job.json"])
		require.True(t, foundFiles["JOB-ACTIVE/logs.txt"])
		require.True(t, foundFiles["JOB-COMPLETED/job.json"])
		require.True(t, foundFiles["JOB-COMPLETED/logs.txt"])
		require.False(t, foundFiles["JOB-IGNORED/job.json"])
	})

	t.Run("Valid Match", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/jobs/archive/bulk?match=Active|Ignored", server.URL))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Read tar.gz
		gzReader, err := gzip.NewReader(resp.Body)
		require.NoError(t, err)
		defer gzReader.Close()

		tarReader := tar.NewReader(gzReader)

		foundFiles := make(map[string]bool)
		for {
			hdr, err := tarReader.Next()
			if err == io.EOF {
				break // End of archive
			}
			require.NoError(t, err)
			foundFiles[hdr.Name] = true
		}

		require.True(t, foundFiles["JOB-ACTIVE/job.json"])
		require.True(t, foundFiles["JOB-ACTIVE/logs.txt"])
		require.False(t, foundFiles["JOB-COMPLETED/job.json"])
		require.True(t, foundFiles["JOB-IGNORED/job.json"])
	})

	t.Run("Valid Status", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/jobs/archive/bulk?status=Failed", server.URL))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Read tar.gz
		gzReader, err := gzip.NewReader(resp.Body)
		require.NoError(t, err)
		defer gzReader.Close()

		tarReader := tar.NewReader(gzReader)

		foundFiles := make(map[string]bool)
		for {
			hdr, err := tarReader.Next()
			if err == io.EOF {
				break // End of archive
			}
			require.NoError(t, err)
			foundFiles[hdr.Name] = true
		}

		require.False(t, foundFiles["JOB-ACTIVE/job.json"])
		require.False(t, foundFiles["JOB-COMPLETED/job.json"])
		require.True(t, foundFiles["JOB-IGNORED/job.json"])
		require.True(t, foundFiles["JOB-IGNORED/logs.txt"])
	})

	t.Run("No Tag or Match or Status", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/jobs/archive/bulk", server.URL))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Invalid Regex", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/jobs/archive/bulk?match=(invalid", server.URL))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
