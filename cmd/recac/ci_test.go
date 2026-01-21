package main

import (
	"bytes"
	"context"
	"os"
	"recac/internal/agent"
	"strings"
	"testing"
)

// SpyAgentCI captures prompts for verification
type SpyAgentCI struct {
	LastPrompt string
	Response   string
}

func (s *SpyAgentCI) Send(ctx context.Context, prompt string) (string, error) {
	s.LastPrompt = prompt
	return s.Response, nil
}

func (s *SpyAgentCI) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	s.LastPrompt = prompt
	if onChunk != nil {
		onChunk(s.Response)
	}
	return s.Response, nil
}

func TestCICmd_DefaultGithub(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	// Create some dummy files to test context generation
	os.WriteFile("go.mod", []byte("module example.com/test"), 0644)
	os.WriteFile("main.go", []byte("package main"), 0644)

	spy := &SpyAgentCI{Response: "name: CI\non: [push]"}

	// Override factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return spy, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute
	cmd := NewCICmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute() // Defaults to github
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify Output File
	content, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	if string(content) != "name: CI\non: [push]" {
		t.Errorf("Unexpected content: %s", string(content))
	}

	// Verify Prompt
	if !strings.Contains(spy.LastPrompt, "go.mod") {
		t.Errorf("Prompt should contain go.mod context")
	}
	if !strings.Contains(spy.LastPrompt, "Target Platform: github") {
		t.Errorf("Prompt should target github")
	}
}

func TestCICmd_CustomPlatform(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	spy := &SpyAgentCI{Response: "gitlab_ci_content"}
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return spy, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute
	cmd := NewCICmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--platform", "gitlab"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify Output File
	content, err := os.ReadFile(".gitlab-ci.yml")
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	if string(content) != "gitlab_ci_content" {
		t.Errorf("Unexpected content: %s", string(content))
	}

	if !strings.Contains(spy.LastPrompt, "Target Platform: gitlab") {
		t.Errorf("Prompt should target gitlab")
	}
}

func TestCICmd_ExistingFile(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	// Create existing file
	os.MkdirAll(".github/workflows", 0755)
	os.WriteFile(".github/workflows/ci.yml", []byte("old"), 0644)

	spy := &SpyAgentCI{Response: "new"}
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return spy, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute without force
	cmd := NewCICmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected error when file exists, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Expected 'already exists' error, got: %v", err)
	}

	// Execute with force
	cmdForce := NewCICmd()
	cmdForce.SetOut(buf)
	cmdForce.SetErr(buf)
	cmdForce.SetArgs([]string{"--force"})

	err = cmdForce.Execute()
	if err != nil {
		t.Fatalf("Execute with --force failed: %v", err)
	}

	content, _ := os.ReadFile(".github/workflows/ci.yml")
	if string(content) != "new" {
		t.Errorf("Expected content to be overwritten to 'new', got '%s'", string(content))
	}
}
