package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestGenerateArchMermaid(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		Components: []architecture.Component{
			{
				ID:          "api-gateway",
				Type:        "service",
				Description: "API Entry Point",
				Consumes:    []architecture.Input{},
				Produces: []architecture.Output{
					{Target: "user-service", Type: "http"},
				},
			},
			{
				ID:          "user-service",
				Type:        "service",
				Description: "Manages users",
				Consumes: []architecture.Input{
					{Source: "api-gateway", Type: "http"},
				},
				Produces: []architecture.Output{
					{Target: "user-db", Type: "sql"},
				},
			},
			{
				ID:          "user-db",
				Type:        "database",
				Description: "Postgres DB",
				Consumes: []architecture.Input{
					{Source: "user-service", Type: "sql"},
				},
			},
		},
	}

	mermaid := generateArchMermaid(arch)

	assert.Contains(t, mermaid, "flowchart TD")

	// Check Nodes
	// api-gateway is service -> ( )
	assert.Contains(t, mermaid, "api_gateway(\"api-gateway")
	// user-service is service -> ( )
	assert.Contains(t, mermaid, "user_service(\"user-service")
	// user-db is database -> [( )]
	assert.Contains(t, mermaid, "user_db[(\"user-db")

	// Check edges
	// Note: cleanArchID replaces "-" with "_"
	// api-gateway -> user-service (http)
	assert.Contains(t, mermaid, "api_gateway -->|http| user_service")
	// user-service -> user-db (sql)
	assert.Contains(t, mermaid, "user_service -->|sql| user_db")

	// Check that we don't have duplicate edges
	// api-gateway -> user-service is defined in both Produces of gateway and Consumes of user-service
	count := strings.Count(mermaid, "api_gateway -->|http| user_service")
	assert.Equal(t, 1, count, "Duplicate edge found")
}

func TestSetupVisualizeServer(t *testing.T) {
	mermaidGraph := "graph TD; A-->B;"
	mux := setupVisualizeServer(mermaidGraph)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body := w.Body.String()
	assert.Contains(t, body, "<title>RECAC Architecture Visualization</title>")
	// Verify content (HTML escaped by template)
	assert.Contains(t, body, "graph TD; A--&gt;B;")
}

func TestRunArchitectVisualize(t *testing.T) {
	// 1. Setup Temp Architecture File
	tmpDir := t.TempDir()
	archFile := filepath.Join(tmpDir, "architecture.yaml")

	yamlContent := `
components:
  - id: service-a
    type: service
    description: Test Service
`
	if err := os.WriteFile(archFile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Mock Dependencies
	originalListen := listenAndServeFunc
	originalOpen := openBrowserFunc
	defer func() {
		listenAndServeFunc = originalListen
		openBrowserFunc = originalOpen
	}()

	listenCalled := false
	listenAndServeFunc = func(addr string, handler http.Handler) error {
		listenCalled = true
		return nil // Return nil to simulate successful start (and immediate return for test)
	}

	openCalled := false
	openBrowserFunc = func(url string) error {
		openCalled = true
		return nil
	}

	// 3. Run Command
	// We construct a command similar to the real one
	cmd := &cobra.Command{Use: "visualize", RunE: runArchitectVisualize}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// We pass the file path as an argument
	err := runArchitectVisualize(cmd, []string{archFile})

	// 4. Verify
	assert.NoError(t, err)
	assert.True(t, listenCalled, "ListenAndServe should be called")

	// Wait a bit for goroutine
	time.Sleep(10 * time.Millisecond)
	assert.True(t, openCalled, "OpenBrowser should be called")

	assert.Contains(t, buf.String(), "Serving architecture visualization")
}
