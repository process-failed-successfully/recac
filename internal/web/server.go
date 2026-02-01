package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"recac/internal/analysis"
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
	workspace string
}

// NewServer creates a new web server
func NewServer(store db.Store, port int, projectID string, workspace string) *Server {
	if projectID == "" {
		projectID = "default"
	}
	if workspace == "" {
		workspace = "."
	}
	return &Server{
		store:     store,
		port:      port,
		projectID: projectID,
		workspace: workspace,
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
	mux.HandleFunc("/api/files", s.handleFiles)
	mux.HandleFunc("/api/file", s.handleFileContent)
	mux.HandleFunc("/api/diagram", s.handleFileDiagram)

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

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(generateMermaid(g)))
}

type FileNode struct {
	Name string     `json:"name"`
	Path string     `json:"path"`
	Type string     `json:"type"` // "file" or "dir"
	Children []FileNode `json:"children,omitempty"`
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	// Simple flat list for now to keep it simple, or recursive?
	// Recursive is better for UI.
	// But let's start with a recursive scan but flatten it? No, let's just return file paths.
	// Actually, for a tree view, a JSON tree is best.

	// Limit depth to prevent massive JSON?
	// Let's limit to 5 levels.

	tree, err := buildFileTree(s.workspace, 0)
	if err != nil {
		http.Error(w, "Failed to scan files: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

func buildFileTree(root string, depth int) ([]FileNode, error) {
	if depth > 5 {
		return nil, nil
	}

	var nodes []FileNode
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.Name() == ".git" || entry.Name() == ".recac" || entry.Name() == "node_modules" || entry.Name() == "vendor" {
			continue
		}

		path := filepath.Join(root, entry.Name())
		// Relative path for client
		// We actually want the full path for server to read, but client just needs to send it back.
		// If we use absolute paths it might be leaky.
		// Ideally we use relative paths from workspace root.

		node := FileNode{
			Name: entry.Name(),
			Path: path,
			Type: "file",
		}

		if entry.IsDir() {
			node.Type = "dir"
			children, err := buildFileTree(path, depth+1)
			if err == nil {
				node.Children = children
			}
		}

		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (s *Server) handleFileContent(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	// Security check: ensure path is within workspace
	// Simple check: absolute path must start with absolute workspace
	absPath, err := filepath.Abs(path)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	absWorkspace, _ := filepath.Abs(s.workspace)
	if !strings.HasPrefix(absPath, absWorkspace) {
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write(content)
}

func (s *Server) handleFileDiagram(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	// Generate diagram for the directory containing the file, or just the file?
	// analyzeStructs walks a root.
	// If we want a diagram for "main.go", we probably want the package it belongs to.

	// If path is a file, take Dir. If dir, take it.
	targetDir := path
	info, err := os.Stat(path)
	if err != nil {
		http.Error(w, "path not found", http.StatusNotFound)
		return
	}
	if !info.IsDir() {
		targetDir = filepath.Dir(path)
	}

	// Security check
	absTarget, _ := filepath.Abs(targetDir)
	absWorkspace, _ := filepath.Abs(s.workspace)
	if !strings.HasPrefix(absTarget, absWorkspace) {
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}

	classes, relationships, err := analysis.AnalyzeStructs(targetDir)
	if err != nil {
		http.Error(w, "analysis failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Focus on the specific file if it was a file?
	// For now, let's show the whole package diagram.

	mermaid := analysis.GenerateMermaidClassDiagram(classes, relationships, nil, true)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(mermaid))
}


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
