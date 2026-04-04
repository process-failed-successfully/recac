package orchestrator

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

func handleSimulate(orch *Orchestrator, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		report := orch.Simulate(logger)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(report); err != nil {
			logger.Error("Failed to encode simulation report", "error", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}

func handleSimulatePipeline(orch *Orchestrator, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		targetJob := r.URL.Query().Get("target")

		items, err := ParsePipelineToWorkItemsWithRunID(body, targetJob, nil, "simulate", "")
		if err != nil {
			logger.Error("Failed to parse pipeline for simulation", "error", err)
			http.Error(w, "Failed to parse pipeline: "+err.Error(), http.StatusBadRequest)
			return
		}

		report := orch.SimulatePipeline(items, logger)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(report); err != nil {
			logger.Error("Failed to encode pipeline simulation report", "error", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}
