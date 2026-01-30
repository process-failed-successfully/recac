package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"recac/internal/db"
	"recac/internal/runner"
)

//go:embed static/*
var staticFiles embed.FS

// Server handles the web visualization
type Server struct {
	store     db.Store
	port      int
	projectID string
}

// NewServer creates a new web server
func NewServer(store db.Store, port int, projectID string) *Server {
	if projectID == "" {
		projectID = "default"
	}
	return &Server{
		store:     store,
		port:      port,
		projectID: projectID,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Static files
	contentStatic, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/", http.FileServer(http.FS(contentStatic)))

	// API endpoints
	mux.HandleFunc("/api/features", s.handleFeatures)
	mux.HandleFunc("/api/graph", s.handleGraph)

	// Bind to localhost for security
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	fmt.Printf("Starting dashboard at http://%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleFeatures(w http.ResponseWriter, r *http.Request) {
	// Query features for the configured project
	content, err := s.store.GetFeatures(s.projectID)
	if err != nil || content == "" {
		// Try to find features for "default" if current project is empty (fallback)
		if s.projectID != "default" {
			content, err = s.store.GetFeatures("default")
		}
	}

	if err != nil || content == "" {
		// SQLite store doesn't easily support "ListProjects", so we might just fail gracefully.
		// Wait, we can assume the user passed the project name in CLI, or we default to what's in DB.
		// Let's return empty list if nothing found.
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}

	var fl db.FeatureList
	if err := json.Unmarshal([]byte(content), &fl); err != nil {
		http.Error(w, "Failed to parse features", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fl.Features)
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	// Similar logic to handleFeatures, we need the task graph.
	content, err := s.store.GetFeatures(s.projectID)
	if err != nil || content == "" {
		if s.projectID != "default" {
			content, err = s.store.GetFeatures("default")
		}
	}

	if err != nil || content == "" {
		w.Write([]byte("graph TD;\nError[No Data Found]"))
		return
	}

	var fl db.FeatureList
	if err := json.Unmarshal([]byte(content), &fl); err != nil {
		w.Write([]byte("graph TD;\nError[Invalid Data]"))
		return
	}

	g := runner.NewTaskGraph()
	if err := g.LoadFromFeatures(fl.Features); err != nil {
		w.Write([]byte("graph TD;\nError[Graph Build Failed]"))
		return
	}

	// Generate Mermaid
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(runner.GenerateMermaid(g)))
}