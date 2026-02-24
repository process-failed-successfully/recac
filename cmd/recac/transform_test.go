package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"recac/internal/agent"
)

// SpyAgentTransform captures prompts for verification
type SpyAgentTransform struct {
	LastPrompt string
	Response   string
}

func (s *SpyAgentTransform) Send(ctx context.Context, prompt string) (string, error) {
	s.LastPrompt = prompt
	return s.Response, nil
}

func (s *SpyAgentTransform) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	s.LastPrompt = prompt
	if onChunk != nil {
		onChunk(s.Response)
	}
	return s.Response, nil
}

func TestTransformCmd_File(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "data.json")
	content := `{"name": "Alice", "age": 30}`
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	spy := &SpyAgentTransform{Response: "name: Alice\nage: 30"}

	// Override factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return spy, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute via RunE directly
	buf := new(bytes.Buffer)
	transformCmd.SetOut(buf)
	transformCmd.SetErr(buf)

	// Reset flags
	transformCmd.Flags().Set("output", "")
	transformOutput = ""

	err = transformCmd.RunE(transformCmd, []string{"convert to yaml", filePath})
	if err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	// Verify
	if !strings.Contains(spy.LastPrompt, content) {
		t.Errorf("Prompt should contain file content. Got: %s", spy.LastPrompt)
	}
	if !strings.Contains(spy.LastPrompt, "convert to yaml") {
		t.Errorf("Prompt should contain instruction. Got: %s", spy.LastPrompt)
	}
	if !strings.Contains(buf.String(), "name: Alice") {
		t.Errorf("Output should contain agent response. Got: %s", buf.String())
	}
}

func TestTransformCmd_Stdin(t *testing.T) {
	// Setup
	input := "user@example.com"
	spy := &SpyAgentTransform{Response: "user@example.com"}

	// Override factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return spy, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute
	buf := new(bytes.Buffer)
	transformCmd.SetIn(strings.NewReader(input))
	transformCmd.SetOut(buf)
	transformCmd.SetErr(buf)

	transformCmd.Flags().Set("output", "")
	transformOutput = ""

	err := transformCmd.RunE(transformCmd, []string{"extract email"})
	if err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	// Verify
	if !strings.Contains(spy.LastPrompt, input) {
		t.Errorf("Prompt should contain stdin content. Got: %s", spy.LastPrompt)
	}
	if !strings.Contains(buf.String(), "user@example.com") {
		t.Errorf("Output should contain agent response. Got: %s", buf.String())
	}
}

func TestTransformCmd_Output(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "output.txt")
	input := "data"

	spy := &SpyAgentTransform{Response: "transformed data"}

	// Override factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return spy, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute
	buf := new(bytes.Buffer)
	transformCmd.SetIn(strings.NewReader(input))
	transformCmd.SetOut(buf)
	transformCmd.SetErr(buf)

	// Manually set flag and variable
	transformCmd.Flags().Set("output", outputFile)
	transformOutput = outputFile
	defer func() {
		transformOutput = ""
		transformCmd.Flags().Set("output", "")
	}()

	err := transformCmd.RunE(transformCmd, []string{"instruction"})
	if err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	// Verify file created
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != "transformed data" {
		t.Errorf("Output file content mismatch. Got: %s, Want: transformed data", string(content))
	}
}
