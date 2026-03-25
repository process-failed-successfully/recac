package orchestrator

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFireJobWebhook(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	t.Run("Success", func(t *testing.T) {
		var receivedJob JobInfo
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			err = json.Unmarshal(body, &receivedJob)
			require.NoError(t, err)

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		job := JobInfo{
			ID: "test-job-1",
			WorkItem: WorkItem{
				WebhookURL: server.URL,
			},
		}

		orch.fireJobWebhook(job, logger)
		assert.Equal(t, "test-job-1", receivedJob.ID)
	})

	t.Run("FailureStatus", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		job := JobInfo{
			ID: "test-job-2",
			WorkItem: WorkItem{
				WebhookURL: server.URL,
			},
		}

		// Should not panic, just logs warning
		orch.fireJobWebhook(job, logger)
	})

	t.Run("InvalidURL", func(t *testing.T) {
		job := JobInfo{
			ID: "test-job-3",
			WorkItem: WorkItem{
				WebhookURL: "http://invalid-url-that-does-not-exist.local",
			},
		}

		// Should not panic, just logs error
		orch.fireJobWebhook(job, logger)
	})

	t.Run("BadURLFormat", func(t *testing.T) {
		job := JobInfo{
			ID: "test-job-4",
			WorkItem: WorkItem{
				WebhookURL: "://bad-format", // causes NewRequest to fail
			},
		}

		// Should not panic, just logs error
		orch.fireJobWebhook(job, logger)
	})
}
