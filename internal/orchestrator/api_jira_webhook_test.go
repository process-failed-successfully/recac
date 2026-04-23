package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestJiraWebhook(t *testing.T) {
	// Setup
	viper.Set("orchestrator.jira_webhook_secret", "my-secret")
	defer viper.Reset()

	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Cancel", mock.Anything, mock.Anything).Return(nil)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)

	orch := New(mockPoller, mockSpawner, 1*time.Minute)
	logger := slog.Default()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())

	t.Run("Valid Webhook Create Issue", func(t *testing.T) {
		orch.CancelAllJobs(context.Background())
		time.Sleep(5 * time.Millisecond)

		payload := map[string]interface{}{
			"webhookEvent": "jira:issue_created",
			"issue": map[string]interface{}{
				"key": "TEST-123",
				"fields": map[string]interface{}{
					"summary":     "Test Issue",
					"description": "Repo: https://github.com/test/repo",
				},
			},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/webhook/jira?secret=my-secret", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Contains(t, w.Body.String(), "Jira Webhook Job TEST-123 submitted successfully")

		// Verify job was created
		jobs := orch.GetActiveJobs()
		if assert.Equal(t, 1, len(jobs)) {
			job := jobs[0]
			assert.Equal(t, "Test Issue", job.Summary)
			assert.Equal(t, "Repo: https://github.com/test/repo", job.WorkItem.Description)
			assert.Equal(t, "https://github.com/test/repo", job.WorkItem.RepoURL)
			assert.Equal(t, "TEST-123", job.WorkItem.EnvVars["JIRA_TICKET"])
			assert.Equal(t, "TEST-123", job.WorkItem.ConcurrencyGroup)
		}
	})

	t.Run("Invalid Secret", func(t *testing.T) {
		orch.CancelAllJobs(context.Background())
		time.Sleep(5 * time.Millisecond)
		payload := map[string]interface{}{
			"webhookEvent": "jira:issue_created",
			"issue": map[string]interface{}{
				"key": "TEST-123",
				"fields": map[string]interface{}{
					"summary":     "Test Issue",
					"description": "Repo: https://github.com/test/repo",
				},
			},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/webhook/jira?secret=invalid-secret", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid secret query parameter")
	})

	t.Run("Missing Secret", func(t *testing.T) {
		orch.CancelAllJobs(context.Background())
		time.Sleep(5 * time.Millisecond)
		payload := map[string]interface{}{
			"webhookEvent": "jira:issue_created",
			"issue": map[string]interface{}{
				"key": "TEST-123",
				"fields": map[string]interface{}{
					"summary":     "Test Issue",
					"description": "Repo: https://github.com/test/repo",
				},
			},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/webhook/jira", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Missing secret query parameter")
	})

	t.Run("Ignored Action", func(t *testing.T) {
		orch.CancelAllJobs(context.Background())
		time.Sleep(5 * time.Millisecond)
		payload := map[string]interface{}{
			"webhookEvent": "jira:issue_deleted",
			"issue": map[string]interface{}{
				"key": "TEST-123",
			},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/webhook/jira?secret=my-secret", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		jobs := orch.GetActiveJobs()
		assert.Len(t, jobs, 0)
	})

	t.Run("Missing Issue Key", func(t *testing.T) {
		orch.CancelAllJobs(context.Background())
		time.Sleep(5 * time.Millisecond)
		payload := map[string]interface{}{
			"webhookEvent": "jira:issue_created",
			"issue": map[string]interface{}{
				"fields": map[string]interface{}{
					"summary": "No ID",
				},
			},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/webhook/jira?secret=my-secret", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Missing issue key or fields")
	})

	t.Run("No Repo URL (Ignored)", func(t *testing.T) {
		orch.CancelAllJobs(context.Background())
		time.Sleep(5 * time.Millisecond)

		payload := map[string]interface{}{
			"webhookEvent": "jira:issue_created",
			"issue": map[string]interface{}{
				"key": "TEST-123",
				"fields": map[string]interface{}{
					"summary":     "Test Issue",
					"description": "No repo here",
				},
			},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/webhook/jira?secret=my-secret", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		jobs := orch.GetActiveJobs()
		assert.Len(t, jobs, 0)
	})
}
