package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/agent"
	"strings"
	"testing"
)

// SpyAgentReview captures prompts for verification
type SpyAgentReview struct {
	LastPrompt string
	Response   string
}

func (s *SpyAgentReview) Send(ctx context.Context, prompt string) (string, error) {
	s.LastPrompt = prompt
	return s.Response, nil
}

func (s *SpyAgentReview) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	s.LastPrompt = prompt
	if onChunk != nil {
		onChunk(s.Response)
	}
	return s.Response, nil
}

func TestReviewCmd_File(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "bad_code.go")
	content := "package main\nfunc main() { var x = 1; }"
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	spy := &SpyAgentReview{Response: "Issues found: Unused variable x."}

	// Override factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return spy, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute
	cmd := NewReviewCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err = cmd.RunE(cmd, []string{filePath})
	if err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	// Verify
	if !strings.Contains(spy.LastPrompt, content) {
		t.Errorf("Prompt should contain file content. Got: %s", spy.LastPrompt)
	}
	if !strings.Contains(buf.String(), "Issues found: Unused variable x") {
		t.Errorf("Output should contain agent response. Got: %s", buf.String())
	}
}

func TestReviewCmd_Diff(t *testing.T) {
	// Check for git
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	// Setup git repo
	tempDir := t.TempDir()

	// Change working directory to tempDir for git commands
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}

	// Init git
	exec.Command("git", "init").Run()
	// Set config to avoid "Please tell me who you are" error
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()
	// Initial commit is needed for HEAD to exist for "git diff HEAD"
	// Create and commit a file
	filePath := "main.go"
	os.WriteFile(filePath, []byte("package main\nfunc main() {}\n"), 0644)
	exec.Command("git", "add", filePath).Run()
	exec.Command("git", "commit", "-m", "Initial commit").Run()

	// Modify the file (unstaged)
	newContent := "package main\nfunc main() { panic(\"oops\") }\n"
	os.WriteFile(filePath, []byte(newContent), 0644)

	spy := &SpyAgentReview{Response: "Critical issue: panic in main."}

	// Override factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return spy, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute
	cmd := NewReviewCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// No args -> should use git diff
	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	// Verify
	if !strings.Contains(spy.LastPrompt, "panic(\"oops\")") {
		t.Errorf("Prompt should contain diff (panic). Got: %s", spy.LastPrompt)
	}
	if !strings.Contains(buf.String(), "Critical issue: panic in main") {
		t.Errorf("Output should contain agent response. Got: %s", buf.String())
	}
}

func TestReviewCmd_NoChanges(t *testing.T) {
	// Check for git
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	// Setup git repo
	tempDir := t.TempDir()

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}

	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	// Execute without commits or changes
	cmd := NewReviewCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.RunE(cmd, []string{})

	if err == nil {
		t.Fatal("Expected error for no changes, got nil")
	}
	if !strings.Contains(err.Error(), "no changes detected") && !strings.Contains(err.Error(), "failed to get git diff") {
		t.Errorf("Unexpected error: %v", err)
	}
}
