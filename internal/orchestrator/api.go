package orchestrator

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/spf13/viper"
)

// RegisterAPI registers the orchestrator API handlers on the provided ServeMux.
func RegisterAPI(mux *http.ServeMux, orch *Orchestrator, logger *slog.Logger, baseCtx context.Context) {
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		status := orch.GetStatus()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			logger.Error("Failed to encode status", "error", err)
		}
	})

	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		var jobs []JobInfo

		switch state {
		case "completed":
			jobs = orch.GetCompletedJobs()
		case "all":
			jobs = append(orch.GetActiveJobs(), orch.GetCompletedJobs()...)
		default:
			jobs = orch.GetActiveJobs()
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(jobs); err != nil {
			logger.Error("Failed to encode jobs", "error", err)
		}
	})

	mux.HandleFunc("GET /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		job, err := orch.GetJob(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(job); err != nil {
			logger.Error("Failed to encode job", "error", err)
		}
	})

	mux.HandleFunc("GET /jobs/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		logStream, err := orch.GetLogs(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		defer logStream.Close()
		io.Copy(w, logStream)
	})

	mux.HandleFunc("POST /jobs/{id}/retry", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		// Use r.Context() but ensure logger is available
		if err := orch.RetryJob(r.Context(), id, logger); err != nil {
			if strings.Contains(err.Error(), "already active") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, "Job %s retry submitted", id)
	})

	mux.HandleFunc("POST /jobs/retry-failed", func(w http.ResponseWriter, r *http.Request) {
		match := r.URL.Query().Get("match")
		count, err := orch.RetryFailedJobs(r.Context(), match, logger)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"retried": %d}`, count)
	})

	mux.HandleFunc("DELETE /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := orch.CancelJob(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Job %s cancellation requested", id)
	})

	mux.HandleFunc("DELETE /jobs", func(w http.ResponseWriter, r *http.Request) {
		count, err := orch.CancelAllJobs(r.Context())
		if err != nil && count == 0 {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"canceled": %d}`, count)
	})

	mux.HandleFunc("DELETE /history", func(w http.ResponseWriter, r *http.Request) {
		count, err := orch.ClearHistory(logger)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to clear history: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"cleared": count})
	})

	mux.HandleFunc("POST /poll", func(w http.ResponseWriter, r *http.Request) {
		orch.ForcePoll()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Orchestrator poll triggered")
	})

	mux.HandleFunc("POST /jobs", func(w http.ResponseWriter, r *http.Request) {
		var item WorkItem
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if item.ID == "" {
			http.Error(w, "Job ID is required", http.StatusBadRequest)
			return
		}

		// Use the baseCtx (captured from main) to ensure job runs independently of the request context
		// but respects orchestrator shutdown.
		if err := orch.SubmitJob(baseCtx, item, logger); err != nil {
			if err == ErrAtCapacity {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
			} else if strings.Contains(err.Error(), "already active") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, "Job %s submitted successfully", item.ID)
	})

	mux.HandleFunc("POST /webhook/github", func(w http.ResponseWriter, r *http.Request) {
		event := r.Header.Get("X-GitHub-Event")
		if event != "issues" && event != "issue_comment" {
			// Ignore other events
			w.WriteHeader(http.StatusOK)
			return
		}

		// Read body for signature validation
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
		// Restore body for any subsequent reading if necessary
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		secret := viper.GetString("orchestrator.github_webhook_secret")
		if secret != "" {
			signature := r.Header.Get("X-Hub-Signature-256")
			if signature == "" {
				http.Error(w, "Missing X-Hub-Signature-256 header", http.StatusUnauthorized)
				return
			}
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(bodyBytes)
			expectedMAC := hex.EncodeToString(mac.Sum(nil))
			if !hmac.Equal([]byte(signature), []byte("sha256="+expectedMAC)) {
				http.Error(w, "Invalid X-Hub-Signature-256 signature", http.StatusUnauthorized)
				return
			}
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		action, _ := payload["action"].(string)
		if action != "opened" && action != "created" && action != "labeled" {
			// We only care about specific actions
			w.WriteHeader(http.StatusOK)
			return
		}

		// Extract fields
		issue, ok := payload["issue"].(map[string]interface{})
		if !ok {
			http.Error(w, "Missing issue in payload", http.StatusBadRequest)
			return
		}
		repoMap, ok := payload["repository"].(map[string]interface{})
		if !ok {
			http.Error(w, "Missing repository in payload", http.StatusBadRequest)
			return
		}

		repoURL, _ := repoMap["clone_url"].(string)
		repoName, _ := repoMap["name"].(string)
		issueNum, _ := issue["number"].(float64)
		title, _ := issue["title"].(string)

		description, _ := issue["body"].(string)

		// If it's a comment, use the comment body and append to description
		if event == "issue_comment" {
			comment, _ := payload["comment"].(map[string]interface{})
			if commentBody, ok := comment["body"].(string); ok {
				description = fmt.Sprintf("Comment:\n%s\n\nIssue Context:\n%s", commentBody, description)
			}
		}

		if repoURL == "" || issueNum == 0 {
			http.Error(w, "Missing required fields (repository.clone_url or issue.number)", http.StatusBadRequest)
			return
		}

		jobID := fmt.Sprintf("gh-%s-%d", repoName, int(issueNum))

		item := WorkItem{
			ID:          jobID,
			Summary:     title,
			Description: description,
			RepoURL:     repoURL,
			EnvVars: map[string]string{
				"GITHUB_ISSUE": fmt.Sprintf("%d", int(issueNum)),
				"GITHUB_REPO":  repoName,
			},
		}

		if err := orch.SubmitJob(baseCtx, item, logger); err != nil {
			if err == ErrAtCapacity {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
			} else if strings.Contains(err.Error(), "already active") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, "GitHub Webhook Job %s submitted successfully", jobID)
	})

	mux.HandleFunc("POST /webhook/gitlab", func(w http.ResponseWriter, r *http.Request) {
		event := r.Header.Get("X-Gitlab-Event")
		if event != "Issue Hook" && event != "Note Hook" {
			// Ignore other events
			w.WriteHeader(http.StatusOK)
			return
		}

		// Read body for validation/parsing
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
		// Restore body for any subsequent reading if necessary
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		secret := viper.GetString("orchestrator.gitlab_webhook_secret")
		if secret != "" {
			token := r.Header.Get("X-Gitlab-Token")
			if token == "" {
				http.Error(w, "Missing X-Gitlab-Token header", http.StatusUnauthorized)
				return
			}
			// Use constant-time comparison to prevent timing attacks
			if !hmac.Equal([]byte(token), []byte(secret)) {
				http.Error(w, "Invalid X-Gitlab-Token header", http.StatusUnauthorized)
				return
			}
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		objectKind, _ := payload["object_kind"].(string)

		// Check if it's an issue or note on an issue
		if objectKind == "note" {
			attr, _ := payload["object_attributes"].(map[string]interface{})
			if noteableType, ok := attr["noteable_type"].(string); !ok || noteableType != "Issue" {
				w.WriteHeader(http.StatusOK)
				return
			}
		} else if objectKind != "issue" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Extract action from object_attributes.action
		attr, ok := payload["object_attributes"].(map[string]interface{})
		if !ok {
			http.Error(w, "Missing object_attributes in payload", http.StatusBadRequest)
			return
		}

		action, _ := attr["action"].(string)
		if objectKind == "issue" && action != "open" && action != "reopen" && action != "update" {
			// We only care about specific actions for issues
			w.WriteHeader(http.StatusOK)
			return
		}

		// For notes, the action might not be explicitly set, but object_kind == note means a comment was made
		// We can proceed.

		// Extract fields
		var issue map[string]interface{}
		if objectKind == "issue" {
			issue = attr
		} else if objectKind == "note" {
			issue, ok = payload["issue"].(map[string]interface{})
			if !ok {
				http.Error(w, "Missing issue in note payload", http.StatusBadRequest)
				return
			}
		}

		projectMap, ok := payload["project"].(map[string]interface{})
		if !ok {
			http.Error(w, "Missing project in payload", http.StatusBadRequest)
			return
		}

		repoURL, _ := projectMap["git_http_url"].(string)
		if repoURL == "" {
			repoURL, _ = projectMap["web_url"].(string) // Fallback
		}

		issueNum, _ := issue["iid"].(float64)
		title, _ := issue["title"].(string)
		description, _ := issue["description"].(string)

		// If it's a comment, use the comment body and append to description
		if objectKind == "note" {
			if commentBody, ok := attr["note"].(string); ok {
				description = fmt.Sprintf("Comment:\n%s\n\nIssue Context:\n%s", commentBody, description)
			}
		}

		if repoURL == "" || issueNum == 0 {
			http.Error(w, "Missing required fields (project.git_http_url or issue.iid)", http.StatusBadRequest)
			return
		}

		jobID := fmt.Sprintf("gl-%d", int(issueNum))

		item := WorkItem{
			ID:          jobID,
			Summary:     title,
			Description: description,
			RepoURL:     repoURL,
			EnvVars: map[string]string{
				"GITLAB_ISSUE": fmt.Sprintf("%d", int(issueNum)),
			},
		}

		if err := orch.SubmitJob(baseCtx, item, logger); err != nil {
			if err == ErrAtCapacity {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
			} else if strings.Contains(err.Error(), "already active") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, "GitLab Webhook Job %s submitted successfully", jobID)
	})

	mux.HandleFunc("POST /pause", func(w http.ResponseWriter, r *http.Request) {
		orch.Pause()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Orchestrator paused")
	})

	mux.HandleFunc("POST /resume", func(w http.ResponseWriter, r *http.Request) {
		orch.Resume()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Orchestrator resumed")
	})
}
