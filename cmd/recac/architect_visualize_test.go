package main

import (
	"net/http"
	"os"
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

func TestRunArchitectVisualize(t *testing.T) {
	// Setup mocks
	originalListen := listenAndServeFunc
	originalOpen := openBrowserForVisFunc
	defer func() {
		listenAndServeFunc = originalListen
		openBrowserForVisFunc = originalOpen
	}()

	listenCalled := false
	openCalled := false

	listenAndServeFunc = func(addr string, handler http.Handler) error {
		listenCalled = true
		assert.Equal(t, ":8080", addr)
		assert.NotNil(t, handler, "handler should be set (ServeMux)")
		return nil // Return immediately
	}

	openBrowserForVisFunc = func(url string) error {
		openCalled = true
		assert.Equal(t, "http://localhost:8080", url)
		return nil
	}

	// Create temp architecture.yaml
	tmpFile, err := os.CreateTemp("", "architecture*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	yamlContent := `
components:
  - id: test-service
    type: service
    description: Test Service
`
	if _, err := tmpFile.Write([]byte(yamlContent)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Run command
	cmd := &cobra.Command{}
	err = runArchitectVisualize(cmd, []string{tmpFile.Name()})
	assert.NoError(t, err)

	assert.True(t, listenCalled, "ListenAndServe should be called")

	// OpenBrowser is called in a goroutine. We need to wait a bit.
	// But since ListenAndServe returns immediately in our mock, the goroutine might not have run yet if scheduler decides so?
	// `go func()` is spawned BEFORE `listenAndServeFunc`.
	// So likely it runs. But to be safe we can use a channel or Eventually.
	// Since we can't modify the goroutine logic easily without injecting a waitgroup,
	// we rely on slight sleep or assume scheduler runs it.
	// Actually, `listenAndServeFunc` returns nil immediately. The main function returns.
	// The goroutine might still be running or not.
	// However, usually `go test` waits for goroutines? No.
	// Let's add a small sleep before checking openCalled.

	time.Sleep(100 * time.Millisecond)
	assert.True(t, openCalled, "OpenBrowser should be called")
}
