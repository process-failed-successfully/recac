package orchestrator

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// handleAnalyzeAnomalies returns a list of jobs whose execution duration or cost
// is an anomaly (> 2 standard deviations from the model mean).
func handleAnalyzeAnomalies(orch *Orchestrator, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		anomalies, err := orch.AnalyzeAnomalies(logger)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Ensure we don't return nil slice as "null"
		if anomalies == nil {
			anomalies = []AnomalyReport{}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(anomalies); err != nil {
			logger.Error("Failed to encode anomalies report", "error", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}
