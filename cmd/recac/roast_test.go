package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// MockRoastAgent implements agent.Agent for testing
type MockRoastAgent struct {
	LastPrompt string
	Response   string
}

func (m *MockRoastAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.LastPrompt = prompt
	return m.Response, nil
}

func (m *MockRoastAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	m.LastPrompt = prompt
	if onChunk != nil {
		onChunk(m.Response)
	}
	return m.Response, nil
}

func TestRoast_SpecificFile(t *testing.T) {
	// 1. Setup Temp File
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "bad_code.go")
	content := "package main\nfunc main() { panic(\"oops\") }"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// 2. Setup Mock Agent
	mockAgent := &MockRoastAgent{
		Response: "You call this code? My grandmother writes better Go.",
	}

	// 3. Override Factory
	originalFactory := roastAgentFactory
	defer func() { roastAgentFactory = originalFactory }()

	roastAgentFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// 4. Run Command
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{Use: "roast", RunE: runRoast}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := runRoast(cmd, []string{filePath})
	if err != nil {
		t.Fatalf("runRoast failed: %v", err)
	}

	// 5. Assertions
	if !strings.Contains(mockAgent.LastPrompt, content) {
		t.Errorf("Agent did not receive file content. Prompt: %s", mockAgent.LastPrompt)
	}

	output := buf.String()
	if !strings.Contains(output, "ROAST OF bad_code.go") {
		t.Errorf("Output missing title. Got: %s", output)
	}
	if !strings.Contains(output, "My grandmother writes better Go") {
		t.Errorf("Output missing roast content. Got: %s", output)
	}
}

func TestRoast_RandomFile(t *testing.T) {
	// 1. Setup Temp Dir with multiple files
	tmpDir := t.TempDir()

	// Create a few files
	files := []string{"a.go", "b.js", "c.py", "ignore.txt"}
	for _, f := range files {
		os.WriteFile(filepath.Join(tmpDir, f), []byte("content of "+f), 0644)
	}

	// 2. Setup Mock Agent
	mockAgent := &MockRoastAgent{
		Response: "Random roast.",
	}

	// 3. Override Factory
	originalFactory := roastAgentFactory
	defer func() { roastAgentFactory = originalFactory }()

	roastAgentFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// 4. Override os.Getwd for the test duration
	originalWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer os.Chdir(originalWd)

	// 5. Run Command
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{Use: "roast", RunE: runRoast}
	cmd.SetOut(buf)

	err := runRoast(cmd, []string{})
	if err != nil {
		t.Fatalf("runRoast failed: %v", err)
	}

	// 6. Assertions
	validContents := []string{"content of a.go", "content of b.js", "content of c.py"}
	found := false
	for _, c := range validContents {
		if strings.Contains(mockAgent.LastPrompt, c) {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Agent prompt did not contain valid file content. Prompt: %s", mockAgent.LastPrompt)
	}

	if strings.Contains(mockAgent.LastPrompt, "content of ignore.txt") {
		t.Errorf("Agent prompt picked ignored file.")
	}
}
