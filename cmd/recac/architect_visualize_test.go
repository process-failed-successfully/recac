package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"recac/internal/architecture"

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
