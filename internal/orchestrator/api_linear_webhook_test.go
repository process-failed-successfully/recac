package orchestrator

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/mock"
)

func TestLinearWebhook(t *testing.T) {
	// Setup
	viper.Set("orchestrator.linear_webhook_secret", "my-secret")
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
		// give orchestrator time to cancel jobs
		time.Sleep(5 * time.Millisecond)
		payload := map[string]interface{}{
			"action": "create",
			"type":   "Issue",
			"data": map[string]interface{}{
				"id":          "issue-123",
				"identifier":  "ENG-123",
				"title":       "Test Issue",
				"description": "Test Description",
			},
			"url": "https://linear.app/team/issue/ENG-123/test-issue",
		}
		body, _ := json.Marshal(payload)

		mac := hmac.New(sha256.New, []byte("my-secret"))
		mac.Write(body)
		signature := hex.EncodeToString(mac.Sum(nil))

		req := httptest.NewRequest("POST", "/webhook/linear", bytes.NewBuffer(body))
		req.Header.Set("Linear-Signature", signature)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Contains(t, w.Body.String(), "Linear Webhook Job ln-issue-123")

		// Verify job was created
		// Use polling since spawning is asynchronous
		var jobs []JobInfo
		for i := 0; i < 20; i++ {
			jobs = orch.GetActiveJobs()
			if len(jobs) == 1 {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}

		require.Len(t, jobs, 1)
		job := jobs[0]
		assert.Equal(t, "Test Issue", job.Summary)
		assert.Equal(t, "Test Description", job.WorkItem.Description)
		assert.Equal(t, "https://linear.app/team/issue/ENG-123/test-issue", job.WorkItem.RepoURL)
		assert.Equal(t, "issue-123", job.WorkItem.EnvVars["LINEAR_ISSUE_ID"])
		assert.Equal(t, "ENG-123", job.WorkItem.EnvVars["LINEAR_ISSUE_IDENTIFIER"])
	})

	t.Run("Invalid Signature", func(t *testing.T) {
		orch.CancelAllJobs(context.Background())
		// give orchestrator time to cancel jobs
		time.Sleep(5 * time.Millisecond)
		payload := map[string]interface{}{
			"action": "create",
			"type":   "Issue",
			"data": map[string]interface{}{
				"id": "issue-123",
			},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/webhook/linear", bytes.NewBuffer(body))
		req.Header.Set("Linear-Signature", "invalid-signature")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid Linear-Signature header")
	})

	t.Run("Missing Signature", func(t *testing.T) {
		orch.CancelAllJobs(context.Background())
		// give orchestrator time to cancel jobs
		time.Sleep(5 * time.Millisecond)
		payload := map[string]interface{}{
			"action": "create",
			"type":   "Issue",
			"data": map[string]interface{}{
				"id": "issue-123",
			},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/webhook/linear", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Missing Linear-Signature header")
	})

	t.Run("Ignored Action", func(t *testing.T) {
		orch.CancelAllJobs(context.Background())
		// give orchestrator time to cancel jobs
		time.Sleep(5 * time.Millisecond)
		payload := map[string]interface{}{
			"action": "remove",
			"type":   "Issue",
			"data": map[string]interface{}{
				"id": "issue-123",
			},
		}
		body, _ := json.Marshal(payload)

		mac := hmac.New(sha256.New, []byte("my-secret"))
		mac.Write(body)
		signature := hex.EncodeToString(mac.Sum(nil))

		req := httptest.NewRequest("POST", "/webhook/linear", bytes.NewBuffer(body))
		req.Header.Set("Linear-Signature", signature)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		jobs := orch.GetActiveJobs()
		assert.Len(t, jobs, 0)
	})

	t.Run("Ignored Type", func(t *testing.T) {
		orch.CancelAllJobs(context.Background())
		// give orchestrator time to cancel jobs
		time.Sleep(5 * time.Millisecond)
		payload := map[string]interface{}{
			"action": "create",
			"type":   "Comment",
			"data": map[string]interface{}{
				"id": "comment-123",
			},
		}
		body, _ := json.Marshal(payload)

		mac := hmac.New(sha256.New, []byte("my-secret"))
		mac.Write(body)
		signature := hex.EncodeToString(mac.Sum(nil))

		req := httptest.NewRequest("POST", "/webhook/linear", bytes.NewBuffer(body))
		req.Header.Set("Linear-Signature", signature)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		jobs := orch.GetActiveJobs()
		assert.Len(t, jobs, 0)
	})

	t.Run("Missing Issue ID", func(t *testing.T) {
		orch.CancelAllJobs(context.Background())
		// give orchestrator time to cancel jobs
		time.Sleep(5 * time.Millisecond)
		payload := map[string]interface{}{
			"action": "create",
			"type":   "Issue",
			"data": map[string]interface{}{
				"title": "No ID",
			},
		}
		body, _ := json.Marshal(payload)

		mac := hmac.New(sha256.New, []byte("my-secret"))
		mac.Write(body)
		signature := hex.EncodeToString(mac.Sum(nil))

		req := httptest.NewRequest("POST", "/webhook/linear", bytes.NewBuffer(body))
		req.Header.Set("Linear-Signature", signature)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Missing required fields (data.id)")
	})
}
