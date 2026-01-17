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

// SpyAgent captures prompts for verification
type SpyAgent struct {
	LastPrompt string
	Response   string
}

func (s *SpyAgent) Send(ctx context.Context, prompt string) (string, error) {
	s.LastPrompt = prompt
	return s.Response, nil
}

func (s *SpyAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	s.LastPrompt = prompt
	if onChunk != nil {
		onChunk(s.Response)
	}
	return s.Response, nil
}

func TestExplainCmd_File(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.go")
	content := "package main\nfunc main() {}"
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	spy := &SpyAgent{Response: "This is a Go main function."}

	// Override factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return spy, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute
	cmd := NewExplainCmd()
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
	if !strings.Contains(buf.String(), "This is a Go main function") {
		t.Errorf("Output should contain agent response. Got: %s", buf.String())
	}
}

func TestExplainCmd_Stdin(t *testing.T) {
	content := "print('hello')"
	spy := &SpyAgent{Response: "Python print statement."}

	// Override factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return spy, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Setup stdin
	bufIn := bytes.NewBufferString(content)

	// Execute
	cmd := NewExplainCmd()
	bufOut := new(bytes.Buffer)
	cmd.SetIn(bufIn)
	cmd.SetOut(bufOut)
	cmd.SetErr(bufOut)

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	// Verify
	if !strings.Contains(spy.LastPrompt, content) {
		t.Errorf("Prompt should contain stdin content. Got: %s", spy.LastPrompt)
	}
	if !strings.Contains(bufOut.String(), "Python print statement") {
		t.Errorf("Output should contain agent response. Got: %s", bufOut.String())
	}
}
