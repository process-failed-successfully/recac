package main

import (
	"context"
	"os"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// MockApiTestAgent for testing
type MockApiTestAgent struct {
	Response string
}

func (m *MockApiTestAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockApiTestAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, nil
}

func TestApiTestCmd(t *testing.T) {
	// 1. Setup Temp Dir with Sample Code
	tmpDir, err := os.MkdirTemp("", "api_test_cmd_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change cwd to tmpDir
	origCwd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer os.Chdir(origCwd)

	content := `
package main
import "net/http"
func main() {
	http.HandleFunc("/api/v1/users", handler)
}
func handler(w http.ResponseWriter, r *http.Request) {}
`
	if err := os.WriteFile("main.go", []byte(content), 0644); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	// 2. Setup Mock Agent
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockResponse := `
//go:build e2e
package e2e

import "testing"

func TestUsers(t *testing.T) {
	// Test code
}
`
	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		return &MockApiTestAgent{Response: mockResponse}, nil
	}

	// 3. Configure Command
	apiTestOutput = "e2e_test.go"
	apiTestBaseURL = "http://test.local"
	apiTestFramework = "std"

	// Create dummy command
	cmd := &cobra.Command{}

	// 4. Run
	err = runApiTest(cmd, []string{})
	if err != nil {
		t.Fatalf("runApiTest failed: %v", err)
	}

	// 5. Verify Output
	outBytes, err := os.ReadFile("e2e_test.go")
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	outStr := string(outBytes)
	if !strings.Contains(outStr, "//go:build e2e") {
		t.Errorf("output missing build tag: %s", outStr)
	}
	if !strings.Contains(outStr, "TestUsers") {
		t.Errorf("output missing test function: %s", outStr)
	}
}
