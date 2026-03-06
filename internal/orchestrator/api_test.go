package orchestrator

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock Spawner (MockPoller is defined in spawner_docker_test.go)
type MockSpawner struct {
	mock.Mock
}

func (m *MockSpawner) Spawn(ctx context.Context, item WorkItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockSpawner) Cancel(ctx context.Context, jobID string) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func (m *MockSpawner) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) {
	args := m.Called(ctx, jobID)
	// Return a dummy ReadCloser
	if r, ok := args.Get(0).(io.ReadCloser); ok {
		return r, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSpawner) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestRegisterAPI(t *testing.T) {
	// Setup
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	orch := New(mockPoller, mockSpawner, 1*time.Minute)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()

	// Register API with mux
	RegisterAPI(mux, orch, logger, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	// 1. Test /status
	t.Run("Status Endpoint", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/status")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var status Status
		err = json.NewDecoder(resp.Body).Decode(&status)
		assert.NoError(t, err)
		assert.Equal(t, "1m0s", status.PollInterval)
	})

	// 2. Test /jobs (empty)
	t.Run("Jobs Endpoint Empty", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/jobs")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var jobs []JobInfo
		err = json.NewDecoder(resp.Body).Decode(&jobs)
		assert.NoError(t, err)
		assert.Empty(t, jobs)
	})

	// 3. Test Submit Job
	item := WorkItem{ID: "JOB-123", Summary: "Test Job"}
	t.Run("Submit Job", func(t *testing.T) {
		body, _ := json.Marshal(item)

		// Expect Spawn call
		mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(i WorkItem) bool {
			return i.ID == "JOB-123"
		})).Return(nil)

		// Expect Cleanup call eventually (but we might not wait for it here)
		mockSpawner.On("Cleanup", mock.Anything, mock.MatchedBy(func(i WorkItem) bool {
			return i.ID == "JOB-123"
		})).Return(nil).Maybe()

		resp, err := http.Post(server.URL + "/jobs", "application/json", strings.NewReader(string(body)))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		// Verify Job is Active or Completed
		time.Sleep(10 * time.Millisecond)

		resp, err = http.Get(server.URL + "/jobs?state=all")
		var jobs []JobInfo
		json.NewDecoder(resp.Body).Decode(&jobs)
		assert.NotEmpty(t, jobs)
		assert.Equal(t, "JOB-123", jobs[0].ID)
	})

	// 3.5 Test Submit Job Too Many Requests
	t.Run("Submit Job Too Many Requests", func(t *testing.T) {
		orch.MaxConcurrentJobs = 1
		// First fill the capacity
		orch.activeSpawns = 1

		item2 := WorkItem{ID: "JOB-456", Summary: "Test Job Too Many"}
		body2, _ := json.Marshal(item2)

		resp, err := http.Post(server.URL + "/jobs", "application/json", strings.NewReader(string(body2)))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

		// Reset for other tests
		orch.activeSpawns = 0
		orch.MaxConcurrentJobs = 0
	})

	// 4. Test Get Job Details
	t.Run("Get Job Details", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/jobs/JOB-123")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var job JobInfo
		err = json.NewDecoder(resp.Body).Decode(&job)
		assert.NoError(t, err)
		assert.Equal(t, "JOB-123", job.ID)
	})

	// 5. Test Get Logs
	t.Run("Get Logs", func(t *testing.T) {
		// Mock GetLogs
		mockLogs := io.NopCloser(strings.NewReader("Log content"))
		mockSpawner.On("GetLogs", mock.Anything, "JOB-123").Return(mockLogs, nil)

		resp, err := http.Get(server.URL + "/jobs/JOB-123/logs")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, "Log content", string(body))
	})

	// 6. Test Cancel Job
	t.Run("Cancel Job", func(t *testing.T) {
		// Expect Cancel call
		mockSpawner.On("Cancel", mock.Anything, "JOB-123").Return(nil)

		req, _ := http.NewRequest(http.MethodDelete, server.URL + "/jobs/JOB-123", nil)
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 6.5 Test Cancel All Jobs
	t.Run("Cancel All Jobs", func(t *testing.T) {
		// Mock Cancel to return nil for any job
		mockSpawner.On("Cancel", mock.Anything, mock.Anything).Return(nil)

		// Insert a dummy active job to be canceled
		orch.mu.Lock()
		orch.activeJobs["JOB-999"] = JobInfo{
			ID: "JOB-999",
			Status: "Running",
		}
		orch.mu.Unlock()

		req, _ := http.NewRequest(http.MethodDelete, server.URL+"/jobs", nil)
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]int
		err = json.NewDecoder(resp.Body).Decode(&result)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, result["canceled"], 1)
	})

	// 7. Test Pause/Resume
	t.Run("Pause Resume", func(t *testing.T) {
		resp, err := http.Post(server.URL + "/pause", "", nil)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		orch.mu.Lock()
		paused := orch.paused
		orch.mu.Unlock()
		assert.True(t, paused)

		resp, err = http.Post(server.URL + "/resume", "", nil)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		orch.mu.Lock()
		paused = orch.paused
		orch.mu.Unlock()
		assert.False(t, paused)
	})

	// 8. Test Retry
	t.Run("Retry Failed", func(t *testing.T) {
		// Manually insert a failed job into history
		orch.mu.Lock()
		orch.completedJobs = append(orch.completedJobs, JobInfo{
			ID: "JOB-FAIL",
			Status: "Failed",
			WorkItem: WorkItem{ID: "JOB-FAIL"},
		})
		orch.mu.Unlock()

		// Expect Spawn call for retry
		mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(i WorkItem) bool {
			return i.ID == "JOB-FAIL"
		})).Return(nil)

		resp, err := http.Post(server.URL + "/jobs/JOB-FAIL/retry", "", nil)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	})

	// 9. Retry All Failed
	t.Run("Retry All Failed", func(t *testing.T) {
		// Manually insert another failed job
		orch.mu.Lock()
		orch.completedJobs = append(orch.completedJobs, JobInfo{
			ID: "JOB-FAIL-2",
			Status: "Failed",
			WorkItem: WorkItem{ID: "JOB-FAIL-2"},
		})
		orch.mu.Unlock()

		// Expect Spawn
		mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(i WorkItem) bool {
			return i.ID == "JOB-FAIL-2"
		})).Return(nil)

		resp, err := http.Post(server.URL + "/jobs/retry-failed", "", nil)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Check response body
		var result map[string]int
		json.NewDecoder(resp.Body).Decode(&result)
		// Should be at least 1 (JOB-FAIL-2)
		assert.GreaterOrEqual(t, result["retried"], 1)
	})

	// 10. Test GitHub Webhook
	t.Run("GitHub Webhook - Issues", func(t *testing.T) {
		viper.Set("orchestrator.github_webhook_secret", "my-secret")
		defer viper.Reset()

		payload := map[string]interface{}{
			"action": "opened",
			"issue": map[string]interface{}{
				"number": float64(42),
				"title":  "Fix bug",
				"body":   "The bug needs fixing",
			},
			"repository": map[string]interface{}{
				"name":      "my-repo",
				"clone_url": "https://github.com/my-org/my-repo.git",
			},
		}
		body, _ := json.Marshal(payload)

		mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(i WorkItem) bool {
			return i.ID == "gh-my-repo-42" &&
				i.Summary == "Fix bug" &&
				i.Description == "The bug needs fixing" &&
				i.RepoURL == "https://github.com/my-org/my-repo.git" &&
				i.EnvVars["GITHUB_ISSUE"] == "42" &&
				i.EnvVars["GITHUB_REPO"] == "my-repo"
		})).Return(nil)

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/webhook/github", strings.NewReader(string(body)))
		req.Header.Set("X-GitHub-Event", "issues")
		req.Header.Set("Content-Type", "application/json")

		mac := hmac.New(sha256.New, []byte("my-secret"))
		mac.Write(body)
		signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Hub-Signature-256", signature)

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	})

	t.Run("GitHub Webhook - Invalid Signature", func(t *testing.T) {
		viper.Set("orchestrator.github_webhook_secret", "my-secret")
		defer viper.Reset()

		payload := map[string]interface{}{"action": "opened"}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/webhook/github", strings.NewReader(string(body)))
		req.Header.Set("X-GitHub-Event", "issues")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hub-Signature-256", "sha256=invalid")

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("GitHub Webhook - Missing Signature", func(t *testing.T) {
		viper.Set("orchestrator.github_webhook_secret", "my-secret")
		defer viper.Reset()

		payload := map[string]interface{}{"action": "opened"}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/webhook/github", strings.NewReader(string(body)))
		req.Header.Set("X-GitHub-Event", "issues")
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("GitHub Webhook - Issue Comment", func(t *testing.T) {
		payload := map[string]interface{}{
			"action": "created",
			"issue": map[string]interface{}{
				"number": float64(43),
				"title":  "Update feature",
				"body":   "The original issue body",
			},
			"comment": map[string]interface{}{
				"body": "Do this specific thing now",
			},
			"repository": map[string]interface{}{
				"name":      "my-repo-2",
				"clone_url": "https://github.com/my-org/my-repo-2.git",
			},
		}
		body, _ := json.Marshal(payload)

		mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(i WorkItem) bool {
			return i.ID == "gh-my-repo-2-43" &&
				i.Summary == "Update feature" &&
				strings.Contains(i.Description, "Do this specific thing now") &&
				strings.Contains(i.Description, "The original issue body") &&
				i.RepoURL == "https://github.com/my-org/my-repo-2.git"
		})).Return(nil)

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/webhook/github", strings.NewReader(string(body)))
		req.Header.Set("X-GitHub-Event", "issue_comment")
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	})

	t.Run("GitHub Webhook - Ignored Event", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/webhook/github", strings.NewReader("{}"))
		req.Header.Set("X-GitHub-Event", "push")

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("GitHub Webhook - Ignored Action", func(t *testing.T) {
		payload := map[string]interface{}{
			"action": "closed",
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/webhook/github", strings.NewReader(string(body)))
		req.Header.Set("X-GitHub-Event", "issues")

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 10.5 Test Force Poll
	t.Run("Force Poll Endpoint", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/poll", nil)
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		assert.Contains(t, string(body), "Orchestrator poll triggered")
	})

	// 11. Test GitLab Webhook
	t.Run("GitLab Webhook - Issue", func(t *testing.T) {
		viper.Set("orchestrator.gitlab_webhook_secret", "my-gitlab-secret")
		defer viper.Reset()

		payload := map[string]interface{}{
			"object_kind": "issue",
			"object_attributes": map[string]interface{}{
				"action": "open",
				"iid":    float64(42),
				"title":  "Test Issue",
				"description": "This is a test issue",
			},
			"project": map[string]interface{}{
				"web_url": "https://gitlab.example.com/owner/repo",
			},
		}
		body, _ := json.Marshal(payload)

		mac := hmac.New(sha256.New, []byte("my-gitlab-secret"))
		mac.Write(body)

		mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(i WorkItem) bool {
			return i.ID == "gl-42" &&
				i.Summary == "Test Issue" &&
				i.Description == "This is a test issue" &&
				i.RepoURL == "https://gitlab.example.com/owner/repo" &&
				i.EnvVars["GITLAB_ISSUE"] == "42"
		})).Return(nil)

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/webhook/gitlab", strings.NewReader(string(body)))
		req.Header.Set("X-Gitlab-Event", "Issue Hook")
		req.Header.Set("X-Gitlab-Token", "my-gitlab-secret")

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	})

	t.Run("GitLab Webhook - Note on Issue", func(t *testing.T) {
		viper.Set("orchestrator.gitlab_webhook_secret", "my-gitlab-secret")
		defer viper.Reset()

		payload := map[string]interface{}{
			"object_kind": "note",
			"object_attributes": map[string]interface{}{
				"noteable_type": "Issue",
				"note": "This is a comment",
			},
			"issue": map[string]interface{}{
				"iid": float64(43),
				"title": "Issue with comment",
				"description": "Original description",
			},
			"project": map[string]interface{}{
				"git_http_url": "https://gitlab.example.com/owner/repo.git",
			},
		}
		body, _ := json.Marshal(payload)

		mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(i WorkItem) bool {
			return i.ID == "gl-43" &&
				i.Summary == "Issue with comment" &&
				strings.Contains(i.Description, "This is a comment") &&
				strings.Contains(i.Description, "Original description") &&
				i.RepoURL == "https://gitlab.example.com/owner/repo.git" &&
				i.EnvVars["GITLAB_ISSUE"] == "43"
		})).Return(nil)

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/webhook/gitlab", strings.NewReader(string(body)))
		req.Header.Set("X-Gitlab-Event", "Note Hook")
		req.Header.Set("X-Gitlab-Token", "my-gitlab-secret")

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	})

	t.Run("GitLab Webhook - Invalid Token", func(t *testing.T) {
		viper.Set("orchestrator.gitlab_webhook_secret", "my-gitlab-secret")
		defer viper.Reset()

		payload := map[string]interface{}{"object_kind": "issue"}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/webhook/gitlab", strings.NewReader(string(body)))
		req.Header.Set("X-Gitlab-Event", "Issue Hook")
		req.Header.Set("X-Gitlab-Token", "wrong-secret")

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("GitLab Webhook - Missing Token", func(t *testing.T) {
		viper.Set("orchestrator.gitlab_webhook_secret", "my-gitlab-secret")
		defer viper.Reset()

		payload := map[string]interface{}{"object_kind": "issue"}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/webhook/gitlab", strings.NewReader(string(body)))
		req.Header.Set("X-Gitlab-Event", "Issue Hook")

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("GitLab Webhook - Ignored Event", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/webhook/gitlab", strings.NewReader("{}"))
		req.Header.Set("X-Gitlab-Event", "Push Hook")

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("GitLab Webhook - Ignored Action", func(t *testing.T) {
		payload := map[string]interface{}{
			"object_kind": "issue",
			"object_attributes": map[string]interface{}{
				"action": "close",
			},
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/webhook/gitlab", strings.NewReader(string(body)))
		req.Header.Set("X-Gitlab-Event", "Issue Hook")

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestAPI_ClearHistory(t *testing.T) {
	orch := New(&mockPoller{}, &mockSpawner{}, 0)
	orch.completedJobs = []JobInfo{{ID: "1"}, {ID: "2"}} // add mock completed jobs

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())

	t.Run("DELETE /history", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, "/history", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var res map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &res)
		assert.NoError(t, err)

		count := res["cleared"].(float64)
		assert.Equal(t, float64(2), count)
		assert.Len(t, orch.GetCompletedJobs(), 0)
	})

	t.Run("GET /history returns 405", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/history", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
	})
}

func TestAPI_ExportJobs(t *testing.T) {
	poller := new(MockPoller)
	spawner := new(MockSpawner)
	spawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)

	orch := New(poller, spawner, 1*time.Minute)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	// Add dummy jobs
	item := WorkItem{
		ID:          "EXPORT-1",
		Summary:     "Summary 1",
		Description: "Desc 1",
		RepoURL:     "url1",
	}
	orch.SubmitJob(ctx, item, logger)

	// It's tricky to directly inject completed jobs because we need to add to history.
	orch.mu.Lock()
	orch.completedJobs = []JobInfo{
		{
			ID:      "COMPLETED-1",
			Summary: "A completed job, with a comma",
			Status:  "Completed",
			WorkItem: WorkItem{
				RepoURL: "url2",
			},
		},
	}
	orch.mu.Unlock()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, ctx)

	t.Run("Export JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/jobs/export?format=json", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		var jobs []JobInfo
		err := json.Unmarshal(rec.Body.Bytes(), &jobs)
		assert.NoError(t, err)
		assert.Len(t, jobs, 2)
	})

	t.Run("Export CSV", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/jobs/export?format=csv", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "text/csv", rec.Header().Get("Content-Type"))

		body := rec.Body.String()
		assert.Contains(t, body, "ID,Summary,Status,StartTime,EndTime,RepoURL")
		assert.Contains(t, body, "EXPORT-1,Summary 1")
		assert.Contains(t, body, "COMPLETED-1,\"A completed job, with a comma\"")
	})
}
