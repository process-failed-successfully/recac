package orchestrator

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
)

// ensureArtifactsDir ensures the base artifacts directory and the job-specific directory exist.
// Returns the path to the job's artifact directory or an error.
func ensureArtifactsDir(o *Orchestrator, jobID string) (string, error) {
	if o.ArtifactsDir == "" {
		return "", fmt.Errorf("artifacts directory is not configured")
	}

	cleanJobID := filepath.Base(jobID)
	if cleanJobID == "." || cleanJobID == ".." || cleanJobID == "/" || cleanJobID != jobID {
		return "", fmt.Errorf("invalid job ID")
	}

	jobDir := filepath.Join(o.ArtifactsDir, cleanJobID)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create artifact directory: %w", err)
	}
	return jobDir, nil
}

// handleUploadArtifact handles PUT /jobs/{id}/artifacts/{filename}
func handleUploadArtifact(o *Orchestrator, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID := r.PathValue("id")
		filename := r.PathValue("filename")

		if jobID == "" || filename == "" {
			http.Error(w, "Missing job ID or filename", http.StatusBadRequest)
			return
		}

		// Ensure directories exist
		jobDir, err := ensureArtifactsDir(o, jobID)
		if err != nil {
			logger.Error("Failed to ensure artifact directory", "error", err, "job_id", jobID)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Prevent path traversal
		cleanFilename := filepath.Base(filename)
		if cleanFilename == "." || cleanFilename == "/" || cleanFilename != filename {
			http.Error(w, "Invalid filename", http.StatusBadRequest)
			return
		}

		filePath := filepath.Join(jobDir, cleanFilename)

		// Create or truncate the file
		file, err := os.Create(filePath)
		if err != nil {
			logger.Error("Failed to create artifact file", "error", err, "path", filePath)
			http.Error(w, "Failed to create artifact file", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Copy body to file
		if _, err := io.Copy(file, r.Body); err != nil {
			logger.Error("Failed to write artifact file", "error", err, "path", filePath)
			http.Error(w, "Failed to write artifact file", http.StatusInternalServerError)
			return
		}

		logger.Info("Artifact uploaded successfully", "job_id", jobID, "filename", cleanFilename)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Artifact %s uploaded successfully.\n", cleanFilename)
	}
}

// handleDownloadArtifact handles GET /jobs/{id}/artifacts/{filename}
func handleDownloadArtifact(o *Orchestrator, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID := r.PathValue("id")
		filename := r.PathValue("filename")

		if jobID == "" || filename == "" {
			http.Error(w, "Missing job ID or filename", http.StatusBadRequest)
			return
		}

		if o.ArtifactsDir == "" {
			http.Error(w, "Artifacts directory is not configured", http.StatusInternalServerError)
			return
		}

		cleanJobID := filepath.Base(jobID)
		if cleanJobID == "." || cleanJobID == ".." || cleanJobID == "/" || cleanJobID != jobID {
			http.Error(w, "Invalid job ID", http.StatusBadRequest)
			return
		}

		cleanFilename := filepath.Base(filename)
		filePath := filepath.Join(o.ArtifactsDir, cleanJobID, cleanFilename)

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			http.Error(w, "Artifact not found", http.StatusNotFound)
			return
		}

		http.ServeFile(w, r, filePath)
	}
}

// handleListArtifacts handles GET /jobs/{id}/artifacts
func handleListArtifacts(o *Orchestrator, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID := r.PathValue("id")

		if jobID == "" {
			http.Error(w, "Missing job ID", http.StatusBadRequest)
			return
		}

		if o.ArtifactsDir == "" {
			http.Error(w, "Artifacts directory is not configured", http.StatusInternalServerError)
			return
		}

		cleanJobID := filepath.Base(jobID)
		if cleanJobID == "." || cleanJobID == ".." || cleanJobID == "/" || cleanJobID != jobID {
			http.Error(w, "Invalid job ID", http.StatusBadRequest)
			return
		}

		jobDir := filepath.Join(o.ArtifactsDir, cleanJobID)

		var artifacts []string

		// Check if directory exists
		if _, err := os.Stat(jobDir); os.IsNotExist(err) {
			// No directory = no artifacts
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string][]string{"artifacts": artifacts})
			return
		}

		entries, err := os.ReadDir(jobDir)
		if err != nil {
			logger.Error("Failed to read artifact directory", "error", err, "job_id", jobID)
			http.Error(w, "Failed to read artifact directory", http.StatusInternalServerError)
			return
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				artifacts = append(artifacts, entry.Name())
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string][]string{"artifacts": artifacts})
	}
}

// handleDeleteArtifact handles DELETE /jobs/{id}/artifacts/{filename}
func handleDeleteArtifact(o *Orchestrator, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID := r.PathValue("id")
		filename := r.PathValue("filename")

		if jobID == "" || filename == "" {
			http.Error(w, "Missing job ID or filename", http.StatusBadRequest)
			return
		}

		if o.ArtifactsDir == "" {
			http.Error(w, "Artifacts directory is not configured", http.StatusInternalServerError)
			return
		}

		cleanJobID := filepath.Base(jobID)
		if cleanJobID == "." || cleanJobID == ".." || cleanJobID == "/" || cleanJobID != jobID {
			http.Error(w, "Invalid job ID", http.StatusBadRequest)
			return
		}

		cleanFilename := filepath.Base(filename)
		filePath := filepath.Join(o.ArtifactsDir, cleanJobID, cleanFilename)

		if err := os.Remove(filePath); err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "Artifact not found", http.StatusNotFound)
			} else {
				logger.Error("Failed to delete artifact", "error", err, "path", filePath)
				http.Error(w, "Failed to delete artifact", http.StatusInternalServerError)
			}
			return
		}

		logger.Info("Artifact deleted successfully", "job_id", jobID, "filename", cleanFilename)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Artifact %s deleted successfully.\n", cleanFilename)
	}
}
