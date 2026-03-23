package main

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// MockRouteAgent for testing
type MockRouteAgent struct {
	Response string
}

func (m *MockRouteAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockRouteAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, nil
}

func TestRoutesCmd(t *testing.T) {
	// 1. Setup Temp Dir with Sample Code
	tmpDir, err := os.MkdirTemp("", "routes_cmd_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := `
package main
import "net/http"
func main() {
	http.HandleFunc("/api/test", handler)
}
func handler(w http.ResponseWriter, r *http.Request) {}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	// 2. Setup Mock Agent
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockResponse := "openapi: 3.0.0\ninfo:\n  title: Generated API\npaths:\n  /api/test:\n    get:\n      summary: Test API\n"
	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		return &MockRouteAgent{Response: mockResponse}, nil
	}

	// Test Case 1: JSON Output to File
	routesFormat = "json"
	routesOutput = filepath.Join(tmpDir, "output.json")

	// Create dummy command
	cmd := &cobra.Command{}

	err = runRoutes(cmd, []string{tmpDir})
	if err != nil {
		t.Fatalf("runRoutes json failed: %v", err)
	}

	// Verify output file
	outBytes, err := os.ReadFile(routesOutput)
	if err != nil {
		t.Fatalf("failed to read output.json: %v", err)
	}
	outStr := string(outBytes)
	if !strings.Contains(outStr, "\"path\": \"/api/test\"") {
		t.Errorf("json output missing route: %s", outStr)
	}

	// Test Case 2: OpenAPI Output to File (using Mock Agent)
	routesFormat = "openapi"
	routesOutput = filepath.Join(tmpDir, "openapi.yaml")

	err = runRoutes(cmd, []string{tmpDir})
	if err != nil {
		t.Fatalf("runRoutes openapi failed: %v", err)
	}

	outBytes, err = os.ReadFile(routesOutput)
	if err != nil {
		t.Fatalf("failed to read openapi.yaml: %v", err)
	}
	outStr = string(outBytes)
	if !strings.Contains(outStr, "openapi: 3.0.0") {
		t.Errorf("openapi output incorrect: %s", outStr)
	}
}

func TestRoutesCmd_TableFormat(t *testing.T) {
	tmpDir := t.TempDir()

	content := `
package main
import "net/http"
func main() {
	http.HandleFunc("/api/table", handlerTable)
}
func handlerTable(w http.ResponseWriter, r *http.Request) {}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	routesFormat = "table"
	routesOutput = filepath.Join(tmpDir, "output.txt")

	cmd := &cobra.Command{}

	err := runRoutes(cmd, []string{tmpDir})
	assert.NoError(t, err)

	outBytes, err := os.ReadFile(routesOutput)
	assert.NoError(t, err)
	outStr := string(outBytes)

	assert.Contains(t, outStr, "METHOD")
	assert.Contains(t, outStr, "PATH")
	assert.Contains(t, outStr, "/api/table")
}

func TestRoutesCmd_InvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()

	content := `
package main
import "net/http"
func main() {
	http.HandleFunc("/api/test", handler)
}
func handler(w http.ResponseWriter, r *http.Request) {}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	routesFormat = "invalid_format_xyz"
	cmd := &cobra.Command{}

	err := runRoutes(cmd, []string{tmpDir})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown format")
}

func TestRoutesCmd_EmptyRoutes(t *testing.T) {
	tmpDir := t.TempDir() // Empty dir

	cmd := &cobra.Command{}

	err := runRoutes(cmd, []string{tmpDir})
	assert.NoError(t, err)
}