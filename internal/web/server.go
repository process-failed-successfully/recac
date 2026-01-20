package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"recac/internal/db"
	"recac/internal/runner"
	"strings"
	"sort"
)

//go:embed static/*
var staticFiles embed.FS

// Server handles the web visualization
type Server struct {
	store db.Store
	port  int
}

// NewServer creates a new web server
func NewServer(store db.Store, port int) *Server {
	return &Server{
		store: store,
		port:  port,
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

	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("Starting dashboard at http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleFeatures(w http.ResponseWriter, r *http.Request) {
	// For now, we assume a default project or try to find one.
	// Since recac is often run in the project root, we can use the root dir name or just "default" project ID
	// if that's how it's stored.
	// However, the CLI usually passes the session name or workspace.
	// We might need to query all projects or accept a query param.
	// For simplicity, let's look for any features.

	// Query 'default' project first
	content, err := s.store.GetFeatures("default")
	if err != nil || content == "" {
		// Try to find ANY project features if default is empty (fallback)
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
	content, err := s.store.GetFeatures("default")
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
	w.Write([]byte(generateMermaid(g)))
}

// generateMermaid matches the logic in cmd/recac/graph.go but reused here
// Ideally we should refactor this into a shared package, but for now I'll duplicate to avoid
// touching existing logic too much as per constraints, or I'll move it to `internal/runner` if I can.
// Actually, `internal/runner/graph.go` is where I should have checked.
// Since I can't easily move it without potentially breaking `cmd/recac/graph.go` (if I move it there),
// I will just copy the helper here for now.
func generateMermaid(g *runner.TaskGraph) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	var nodes []*runner.TaskNode
	for _, node := range g.Nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	for _, node := range nodes {
		style := ""
		switch node.Status {
		case runner.TaskDone:
			style = ":::done"
		case runner.TaskInProgress:
			style = ":::inprogress"
		case runner.TaskFailed:
			style = ":::failed"
		case runner.TaskReady:
			style = ":::ready"
		default:
			style = ":::pending"
		}

		safeID := sanitizeMermaidID(node.ID)
		safeName := strings.ReplaceAll(node.Name, "\"", "'")
		safeName = strings.ReplaceAll(safeName, "\n", " ")
		if len(safeName) > 30 {
			safeName = safeName[:27] + "..."
		}

		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]%s\n", safeID, safeName, style))

		for _, depID := range node.Dependencies {
			safeDepID := sanitizeMermaidID(depID)
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeDepID, safeID))
		}
	}

	sb.WriteString("\n    classDef done fill:#90EE90,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef inprogress fill:#87CEEB,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef failed fill:#FF6347,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef ready fill:#FFD700,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef pending fill:#D3D3D3,stroke:#333,stroke-width:1px,color:black;\n")

	return sb.String()
}

func sanitizeMermaidID(id string) string {
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, ".", "_")
	return id
}
