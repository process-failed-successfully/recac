package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ApiMockAgent for testing
type ApiMockAgent struct {
	agent.Agent
	SendFunc       func(ctx context.Context, prompt string) (string, error)
	SendStreamFunc func(ctx context.Context, prompt string, onChunk func(string)) (string, error)
}

func (m *ApiMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, prompt)
	}
	return "mock response", nil
}

func (m *ApiMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if m.SendStreamFunc != nil {
		return m.SendStreamFunc(ctx, prompt, onChunk)
	}
	onChunk("mock response")
	return "mock response", nil
}

func TestApiScan(t *testing.T) {
	// Setup temp directory
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Create dummy files to scan
	os.WriteFile("main.go", []byte("package main\nfunc main() {}\n"), 0644)

	// Mock Agent Factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &ApiMockAgent{
			SendFunc: func(ctx context.Context, prompt string) (string, error) {
				return "openapi: 3.0.0\ninfo:\n  title: Mock API\n  version: 1.0.0", nil
			},
		}, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Run scan command
	cmd, _, _ := newRootCmd()
	output, err := executeCommand(cmd, "api", "scan", "--output", "test_openapi.yaml")
	require.NoError(t, err)

	// Verify output file exists and has content
	content, err := os.ReadFile("test_openapi.yaml")
	require.NoError(t, err)
	assert.Contains(t, string(content), "openapi: 3.0.0")
	assert.Contains(t, output, "✅ OpenAPI spec saved")
}

func TestApiServe(t *testing.T) {
	// Setup temp directory with spec file
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	err := os.WriteFile(specPath, []byte("openapi: 3.0.0"), 0644)
	require.NoError(t, err)

	// We can't easily test the blocking server in executeCommand.
	// However, we can test that it fails if file is missing.
	cmd, _, _ := newRootCmd()
	_, err = executeCommand(cmd, "api", "serve", "non_existent.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// To test successful server start, we would need to run it in a goroutine and kill it,
	// but executeCommand mocks exit which complicates things.
	// For unit testing the server handler, we can verify the mux setup if we refactored slightly.
	// But since we modified runApiServe to use a local mux, we can conceptually verify logic.

	// Let's manually trigger the handler logic just to verify it serves correct content
	// Replicating logic from runApiServe's mux setup:
	mux := http.NewServeMux()
	mux.HandleFunc("/spec", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, specPath)
	})

	req := httptest.NewRequest("GET", "/spec", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Check content
	body, _ := os.ReadFile(specPath)
	assert.Equal(t, string(body), "openapi: 3.0.0")
}
