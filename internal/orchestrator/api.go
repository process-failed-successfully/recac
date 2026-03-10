package orchestrator

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	encoding_csv "encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// RegisterAPI registers the orchestrator API handlers on the provided ServeMux.
func RegisterAPI(mux *http.ServeMux, orch *Orchestrator, logger *slog.Logger, baseCtx context.Context) {
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(DashboardHTML))
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("GET /events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		ch := orch.Subscribe()
		defer orch.Unsubscribe(ch)

		// Send an initial connected event
		fmt.Fprintf(w, "data: {\"event\": \"connected\"}\n\n")
		flusher.Flush()

		for {
			select {
			case <-r.Context().Done():
				return
			case eventData := <-ch:
				fmt.Fprintf(w, "data: %s\n\n", string(eventData))
				flusher.Flush()
			}
		}
	})

	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		status := orch.GetStatus()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			logger.Error("Failed to encode status", "error", err)
		}
	})

	mux.HandleFunc("GET /analytics", func(w http.ResponseWriter, r *http.Request) {
		analytics := orch.GetAnalytics()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(analytics); err != nil {
			logger.Error("Failed to encode analytics", "error", err)
		}
	})

	mux.HandleFunc("GET /jobs", func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		statusFilter := r.URL.Query().Get("status")
		tagFilter := r.URL.Query().Get("tag")
		var jobs []JobInfo

		switch state {
		case "completed":
			jobs = orch.GetCompletedJobs()
		case "pending":
			jobs = orch.GetPendingJobs()
		case "all":
			jobs = append(orch.GetActiveJobs(), orch.GetCompletedJobs()...)
		default:
			jobs = orch.GetActiveJobs()
		}

		if statusFilter != "" {
			var filtered []JobInfo
			lowerStatusFilter := strings.ToLower(statusFilter)
			for _, job := range jobs {
				if strings.ToLower(job.Status) == lowerStatusFilter {
					filtered = append(filtered, job)
				}
			}
			jobs = filtered
		}

		if tagFilter != "" {
			var filtered []JobInfo
			lowerTagFilter := strings.ToLower(tagFilter)
			for _, job := range jobs {
				hasTag := false
				for _, tag := range job.WorkItem.Tags {
					if strings.ToLower(tag) == lowerTagFilter {
						hasTag = true
						break
					}
				}
				if hasTag {
					filtered = append(filtered, job)
				}
			}
			jobs = filtered
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(jobs); err != nil {
			logger.Error("Failed to encode jobs", "error", err)
		}
	})

	mux.HandleFunc("GET /jobs/export", func(w http.ResponseWriter, r *http.Request) {
		format := r.URL.Query().Get("format")
		jobs := append(orch.GetActiveJobs(), orch.GetCompletedJobs()...)

		if format == "csv" {
			w.Header().Set("Content-Type", "text/csv")
			w.Header().Set("Content-Disposition", "attachment; filename=jobs_export.csv")

			writer := encoding_csv.NewWriter(w)
			defer writer.Flush()

			// Header
			writer.Write([]string{"ID", "Summary", "Status", "StartTime", "EndTime", "RepoURL"})

			for _, job := range jobs {
				startTime := ""
				if !job.StartTime.IsZero() {
					startTime = job.StartTime.Format(time.RFC3339)
				}

				endTime := ""
				if !job.EndTime.IsZero() {
					endTime = job.EndTime.Format(time.RFC3339)
				}

				writer.Write([]string{
					job.ID,
					job.Summary,
					job.Status,
					startTime,
					endTime,
					job.WorkItem.RepoURL,
				})
			}
		} else {
			// Default to JSON
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Disposition", "attachment; filename=jobs_export.json")
			if err := json.NewEncoder(w).Encode(jobs); err != nil {
				logger.Error("Failed to encode jobs for export", "error", err)
			}
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

	mux.HandleFunc("POST /jobs/{id}/output", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			Outputs map[string]string `json:"outputs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if err := orch.SetJobOutput(id, req.Outputs, logger); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": "success", "job_id": "%s"}`, id)
	})

	mux.HandleFunc("PUT /jobs/{id}/dependencies", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			DependsOn []string `json:"depends_on"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if err := orch.UpdateJobDependencies(r.Context(), id, req.DependsOn, logger); err != nil {
			if strings.Contains(err.Error(), "already active") || strings.Contains(err.Error(), "already completed") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "circular dependency") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		respData, _ := json.Marshal(req)
		w.Write(respData)
	})

	mux.HandleFunc("PUT /jobs/{id}/priority", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			Priority int `json:"priority"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if err := orch.UpdateJobPriority(r.Context(), id, req.Priority, logger); err != nil {
			if strings.Contains(err.Error(), "already active") || strings.Contains(err.Error(), "already completed") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"priority": %d}`, req.Priority)
	})

	mux.HandleFunc("PUT /jobs/{id}/timeout", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			Timeout string `json:"timeout"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		parsedTimeout, err := time.ParseDuration(req.Timeout)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid timeout format: %v", err), http.StatusBadRequest)
			return
		}

		if err := orch.UpdateJobTimeout(r.Context(), id, parsedTimeout, logger); err != nil {
			if strings.Contains(err.Error(), "already active") || strings.Contains(err.Error(), "already completed") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"timeout": "%s"}`, parsedTimeout.String())
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

	mux.HandleFunc("POST /jobs/{id}/approve", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := orch.ApproveJob(r.Context(), id, logger); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else if strings.Contains(err.Error(), "not pending approval") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Job %s approved", id)
	})

	mux.HandleFunc("POST /jobs/{id}/clone", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var overrides struct {
			NewID     string            `json:"new_id"`
			EnvVars   map[string]string `json:"env_vars"`
			Priority  *int              `json:"priority"`
			DependsOn []string          `json:"depends_on"`
		}

		if r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil && len(bodyBytes) > 0 {
				if err := json.Unmarshal(bodyBytes, &overrides); err != nil {
					http.Error(w, "Invalid JSON body", http.StatusBadRequest)
					return
				}
			}
		}

		job, err := orch.GetJob(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		newItem := job.WorkItem

		if overrides.NewID != "" {
			newItem.ID = overrides.NewID
		} else {
			newItem.ID = fmt.Sprintf("%s-clone-%d", newItem.ID, time.Now().UnixNano())
		}

		if job.WorkItem.EnvVars != nil {
			newItem.EnvVars = make(map[string]string)
			for k, v := range job.WorkItem.EnvVars {
				newItem.EnvVars[k] = v
			}
		}
		if overrides.EnvVars != nil {
			if newItem.EnvVars == nil {
				newItem.EnvVars = make(map[string]string)
			}
			for k, v := range overrides.EnvVars {
				newItem.EnvVars[k] = v
			}
		}

		if overrides.Priority != nil {
			newItem.Priority = *overrides.Priority
		}

		if overrides.DependsOn != nil {
			newItem.DependsOn = make([]string, len(overrides.DependsOn))
			copy(newItem.DependsOn, overrides.DependsOn)
		} else if job.WorkItem.DependsOn != nil {
			newItem.DependsOn = make([]string, len(job.WorkItem.DependsOn))
			copy(newItem.DependsOn, job.WorkItem.DependsOn)
		}

		if err := orch.SubmitJob(baseCtx, newItem, logger); err != nil {
			if err == ErrAtCapacity {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
			} else if strings.Contains(err.Error(), "already active") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `{"cloned_job_id": "%s"}`, newItem.ID)
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
		tag := r.URL.Query().Get("tag")
		var count int
		var err error

		if tag != "" {
			count, err = orch.CancelJobsByTag(r.Context(), tag, logger)
		} else {
			count, err = orch.CancelAllJobs(r.Context())
		}

		if err != nil && count == 0 {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"canceled": %d}`, count)
	})

	mux.HandleFunc("DELETE /pending", func(w http.ResponseWriter, r *http.Request) {
		count := orch.ClearPendingJobs(r.Context(), logger)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"cleared": %d}`, count)
	})

	mux.HandleFunc("DELETE /history/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := orch.PurgeJob(id, logger); err != nil {
			if strings.Contains(err.Error(), "cannot purge") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Job %s purged successfully", id)
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
			} else if err == ErrDraining {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
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

	mux.HandleFunc("POST /jobs/batch", func(w http.ResponseWriter, r *http.Request) {
		var items []WorkItem
		if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
			http.Error(w, "Invalid JSON array body", http.StatusBadRequest)
			return
		}

		for _, item := range items {
			if item.ID == "" {
				http.Error(w, "Job ID is required for all jobs in batch", http.StatusBadRequest)
				return
			}
		}

		var submitted []string
		var errors []string

		for _, item := range items {
			if err := orch.SubmitJob(baseCtx, item, logger); err != nil {
				if err == ErrAtCapacity {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "at capacity"))
				} else if err == ErrDraining {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "draining"))
				} else if strings.Contains(err.Error(), "already active") {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "already active"))
				} else {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, err.Error()))
				}
			} else {
				submitted = append(submitted, item.ID)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"submitted": submitted,
			"errors":    errors,
		}); err != nil {
			logger.Error("Failed to encode batch submission response", "error", err)
		}
	})

	mux.HandleFunc("POST /jobs/matrix", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			BaseItem WorkItem            `json:"base_item"`
			Matrix   map[string][]string `json:"matrix"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if req.BaseItem.ID == "" {
			http.Error(w, "Base Job ID is required", http.StatusBadRequest)
			return
		}

		// Generate combinations deterministically (sort keys)
		var keys []string
		for k := range req.Matrix {
			keys = append(keys, k)
		}

		// Sort keys to ensure deterministic combination generation
		sort.Strings(keys)

		var combinations []map[string]string
		var generate func(int, map[string]string)
		generate = func(idx int, current map[string]string) {
			if idx == len(keys) {
				cp := make(map[string]string)
				for k, v := range current {
					cp[k] = v
				}
				combinations = append(combinations, cp)
				return
			}
			key := keys[idx]
			for _, val := range req.Matrix[key] {
				current[key] = val
				generate(idx+1, current)
			}
		}
		generate(0, make(map[string]string))

		// If matrix is empty, just submit the base item
		if len(combinations) == 0 {
			combinations = append(combinations, make(map[string]string))
		}

		var submitted []string
		var errors []string

		for i, combo := range combinations {
			item := req.BaseItem // shallow copy

			// deep copy EnvVars
			item.EnvVars = make(map[string]string)
			for k, v := range req.BaseItem.EnvVars {
				item.EnvVars[k] = v
			}

			// Add matrix variables
			suffixParts := []string{}
			for _, k := range keys {
				if v, ok := combo[k]; ok {
					item.EnvVars[k] = v
					suffixParts = append(suffixParts, fmt.Sprintf("%s=%s", k, v))
				}
			}

			if len(suffixParts) > 0 {
				item.ID = fmt.Sprintf("%s-%d", req.BaseItem.ID, i+1)
				item.Summary = fmt.Sprintf("%s [%s]", req.BaseItem.Summary, strings.Join(suffixParts, ", "))
			}

			// Deep copy DependsOn
			if req.BaseItem.DependsOn != nil {
				item.DependsOn = make([]string, len(req.BaseItem.DependsOn))
				copy(item.DependsOn, req.BaseItem.DependsOn)
			}
			// Deep copy Tags
			if req.BaseItem.Tags != nil {
				item.Tags = make([]string, len(req.BaseItem.Tags))
				copy(item.Tags, req.BaseItem.Tags)
			}

			if err := orch.SubmitJob(baseCtx, item, logger); err != nil {
				if err == ErrAtCapacity {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "at capacity"))
				} else if err == ErrDraining {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "draining"))
				} else if strings.Contains(err.Error(), "already active") {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "already active"))
				} else {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, err.Error()))
				}
			} else {
				submitted = append(submitted, item.ID)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"submitted": submitted,
			"errors":    errors,
		}); err != nil {
			logger.Error("Failed to encode matrix submission response", "error", err)
		}
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

		concurrencyGroup := fmt.Sprintf("gh-%s-%d", repoName, int(issueNum))
		jobID := fmt.Sprintf("%s-%d", concurrencyGroup, time.Now().UnixNano())

		item := WorkItem{
			ID:               jobID,
			Summary:          title,
			Description:      description,
			RepoURL:          repoURL,
			ConcurrencyGroup: concurrencyGroup,
			CancelInProgress: true,
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

		concurrencyGroup := fmt.Sprintf("gl-%d", int(issueNum))
		jobID := fmt.Sprintf("%s-%d", concurrencyGroup, time.Now().UnixNano())

		item := WorkItem{
			ID:               jobID,
			Summary:          title,
			Description:      description,
			RepoURL:          repoURL,
			ConcurrencyGroup: concurrencyGroup,
			CancelInProgress: true,
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

	mux.HandleFunc("POST /drain", func(w http.ResponseWriter, r *http.Request) {
		orch.Drain(logger)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Orchestrator draining")
	})

	mux.HandleFunc("POST /undrain", func(w http.ResponseWriter, r *http.Request) {
		orch.Undrain(logger)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Orchestrator undraining")
	})

	mux.HandleFunc("POST /scale", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MaxConcurrentJobs int `json:"max_concurrent_jobs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		// Prevent negative numbers
		if req.MaxConcurrentJobs < 0 {
			http.Error(w, "max_concurrent_jobs cannot be negative", http.StatusBadRequest)
			return
		}

		orch.SetConcurrency(baseCtx, req.MaxConcurrentJobs, logger)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"max_concurrent_jobs": %d}`, req.MaxConcurrentJobs)
	})
}
