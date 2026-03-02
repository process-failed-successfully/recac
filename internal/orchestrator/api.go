package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
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
			if strings.Contains(err.Error(), "already active") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, "Job %s submitted successfully", item.ID)
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
