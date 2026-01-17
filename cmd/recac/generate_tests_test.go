package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"strings"
	"testing"
)

// SpyAgentGen captures prompts for verification
type SpyAgentGen struct {
	LastPrompt string
	Response   string
}

func (s *SpyAgentGen) Send(ctx context.Context, prompt string) (string, error) {
	s.LastPrompt = prompt
	return s.Response, nil
}

func (s *SpyAgentGen) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	s.LastPrompt = prompt
	if onChunk != nil {
		onChunk(s.Response)
	}
	return s.Response, nil
}

func TestGenerateTestsCmd_File(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "calc.go")
	content := "package main\nfunc Add(a, b int) int { return a + b }"
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	spy := &SpyAgentGen{Response: "```go\nfunc TestAdd(t *testing.T) {}\n```"}

	// Override factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return spy, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute
	cmd := NewGenerateTestsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// We must reset flags if they are dirty from previous runs,
	// but NewGenerateTestsCmd creates a new command with new flags each time.

	err = cmd.RunE(cmd, []string{filePath})
	if err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	// Verify
	if !strings.Contains(spy.LastPrompt, content) {
		t.Errorf("Prompt should contain file content. Got: %s", spy.LastPrompt)
	}
	if !strings.Contains(spy.LastPrompt, "Infer the best testing framework") {
		t.Errorf("Prompt should ask to infer framework by default. Got: %s", spy.LastPrompt)
	}
	if !strings.Contains(buf.String(), "func TestAdd") {
		t.Errorf("Output should contain agent response. Got: %s", buf.String())
	}
}

func TestGenerateTestsCmd_Framework(t *testing.T) {
	// Setup
	spy := &SpyAgentGen{Response: "test code"}
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return spy, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute
	cmd := NewGenerateTestsCmd()
	buf := new(bytes.Buffer)
	cmd.SetIn(strings.NewReader("code"))
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"--framework", "pytest"})

	err := cmd.Execute() // Use Execute to parse flags
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify
	if !strings.Contains(spy.LastPrompt, "pytest") {
		t.Errorf("Prompt should request pytest framework. Got: %s", spy.LastPrompt)
	}
}

func TestGenerateTestsCmd_Output(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "input.go")
	os.WriteFile(filePath, []byte("code"), 0644)
	outputFile := filepath.Join(tempDir, "output_test.go")

	spy := &SpyAgentGen{Response: "Here is the code:\n```go\nfunc TestSomething() {}\n```\nHope it helps."}

	// Override factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return spy, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute
	cmd := NewGenerateTestsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// We need to simulate flag parsing or set flags manually
	// cmd.Execute() parses flags from os.Args by default unless cmd.SetArgs is used.
	cmd.SetArgs([]string{filePath, "--output", outputFile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify file created
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	stringContent := string(content)
	if !strings.Contains(stringContent, "func TestSomething()") {
		t.Errorf("Output file missing code. Got: %s", stringContent)
	}
	if strings.Contains(stringContent, "Here is the code") {
		t.Errorf("Output file should not contain conversational text. Got: %s", stringContent)
	}
	if strings.Contains(stringContent, "```") {
		t.Errorf("Output file should not contain markdown backticks. Got: %s", stringContent)
	}
}

func TestExtractCodeBlock(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "```go\nfunc main() {}\n```",
			expected: "func main() {}",
		},
		{
			input:    "Prefix\n```\ncode\n```\nSuffix",
			expected: "code",
		},
		{
			input:    "No blocks",
			expected: "No blocks",
		},
		{
			input:    "```\nUnclosed",
			expected: "Unclosed",
		},
	}

	for _, tt := range tests {
		got := extractCodeBlock(tt.input)
		if strings.TrimSpace(got) != strings.TrimSpace(tt.expected) {
			t.Errorf("extractCodeBlock(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}
