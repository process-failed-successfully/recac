package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
)

func TestApiScan(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "recac-api-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Create Sample Go File
	goFile := filepath.Join(tmpDir, "server.go")
	content := `package main

import (
	"fmt"
	"net/http"
)

func handleRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello")
}

func main() {
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("users"))
	})
	http.Handle("/metrics", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Mock Agent
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse("Mock description")

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// 4. Run Command (No Describe)
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Ensure apiDescribe is reset after test
	defer func() { apiDescribe = false }()

	// Force disable describe first
	apiDescribe = false

	err = runApiScan(cmd, []string{tmpDir})
	if err != nil {
		t.Fatalf("runApiScan failed: %v", err)
	}

	output := buf.String()
	// Check for expected output
	if !strings.Contains(output, "/") {
		t.Errorf("Expected output to contain root path '/', got:\n%s", output)
	}
	if !strings.Contains(output, "handleRoot") {
		t.Errorf("Expected output to contain 'handleRoot', got:\n%s", output)
	}
	if !strings.Contains(output, "/api/v1/users") {
		t.Errorf("Expected output to contain '/api/v1/users', got:\n%s", output)
	}
	if !strings.Contains(output, "func(...)") {
		t.Errorf("Expected output to contain 'func(...)', got:\n%s", output)
	}

	// 5. Run Command (With Describe)
	buf.Reset()
	apiDescribe = true

	err = runApiScan(cmd, []string{tmpDir})
	if err != nil {
		t.Fatalf("runApiScan with describe failed: %v", err)
	}

	output = buf.String()
	if !strings.Contains(output, "Mock description") {
		t.Errorf("Expected output to contain 'Mock description', got:\n%s", output)
	}
}
