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

// SpyAgentTranslate captures prompts for verification.
// Using a distinct name to avoid collision with SpyAgent in explain_test.go if verified together.
type SpyAgentTranslate struct {
	LastPrompt string
	Response   string
}

func (s *SpyAgentTranslate) Send(ctx context.Context, prompt string) (string, error) {
	s.LastPrompt = prompt
	return s.Response, nil
}

func (s *SpyAgentTranslate) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	s.LastPrompt = prompt
	if onChunk != nil {
		onChunk(s.Response)
	}
	return s.Response, nil
}

func TestTranslateCmd_File(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "source.py")
	content := "print('hello')"
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	spy := &SpyAgentTranslate{Response: "fmt.Println(\"hello\")"}

	// Override factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return spy, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute
	cmd := NewTranslateCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Reset flags
	translateTo = "go"
	translateFrom = ""
	translateOut = ""
	translateCode = ""

	err = cmd.RunE(cmd, []string{filePath})
	if err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	// Verify
	if !strings.Contains(spy.LastPrompt, content) {
		t.Errorf("Prompt should contain file content. Got: %s", spy.LastPrompt)
	}
	if !strings.Contains(spy.LastPrompt, "to go") {
		t.Errorf("Prompt should contain target language. Got: %s", spy.LastPrompt)
	}
	if !strings.Contains(buf.String(), "fmt.Println(\"hello\")") {
		t.Errorf("Output should contain agent response. Got: %s", buf.String())
	}
}

func TestTranslateCmd_CodeFlag(t *testing.T) {
	content := "console.log('hi')"
	spy := &SpyAgentTranslate{Response: "print('hi')"}

	// Override factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return spy, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute
	cmd := NewTranslateCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Set flags
	translateTo = "python"
	translateFrom = "js"
	translateCode = content
	translateOut = ""

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	// Verify
	if !strings.Contains(spy.LastPrompt, content) {
		t.Errorf("Prompt should contain code content. Got: %s", spy.LastPrompt)
	}
	if !strings.Contains(spy.LastPrompt, "from js") {
		t.Errorf("Prompt should contain source language. Got: %s", spy.LastPrompt)
	}
	if !strings.Contains(buf.String(), "print('hi')") {
		t.Errorf("Output should contain agent response. Got: %s", buf.String())
	}
}

func TestTranslateCmd_OutFile(t *testing.T) {
	content := "foo"
	spy := &SpyAgentTranslate{Response: "bar"}

	// Override factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return spy, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	tempDir := t.TempDir()
	outFile := filepath.Join(tempDir, "out.txt")

	// Execute
	cmd := NewTranslateCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Set flags
	translateTo = "lang2"
	translateCode = content
	translateOut = outFile
	translateFrom = ""

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	// Verify file created
	outContent, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	if string(outContent) != "bar" {
		t.Errorf("Output file content mismatch. Got: %s, Want: bar", string(outContent))
	}
}

func TestTranslateCmd_MissingTo(t *testing.T) {
	cmd := NewTranslateCmd()

	// Reset flags
	translateTo = ""
	translateCode = "code"

	err := cmd.RunE(cmd, []string{})
	if err == nil {
		t.Error("Expected error when --to is missing")
	}
	if err != nil && !strings.Contains(err.Error(), "target language is required") {
		t.Errorf("Expected specific error message. Got: %v", err)
	}
}
