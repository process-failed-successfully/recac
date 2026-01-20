package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"strings"
	"testing"
)

// SequentialMockAgent returns a sequence of responses
type SequentialMockAgent struct {
	Responses []string
	CallCount int
}

func (s *SequentialMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if s.CallCount >= len(s.Responses) {
		return "", errors.New("no more responses")
	}
	resp := s.Responses[s.CallCount]
	s.CallCount++
	return resp, nil
}

func (s *SequentialMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return s.Send(ctx, prompt)
}

func TestGenerateTestsCmd_AutoFix(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "buggy.go")
	os.WriteFile(filePath, []byte("buggy code"), 0644)
	outputFile := filepath.Join(tempDir, "buggy_test.go")

	// Mock Agent: Returns bad test first, then good test
	mockAgent := &SequentialMockAgent{
		Responses: []string{
			"```go\nfunc TestBad() { panic(\"fail\") }\n```",
			"```go\nfunc TestGood() { /* pass */ }\n```",
		},
	}

	// Override Agent Factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Override Shell Command
	originalExec := executeShellCommand
	callCount := 0
	executeShellCommand = func(command string) (string, error) {
		callCount++
		if callCount == 1 {
			return "FAIL: TestBad", errors.New("exit status 1")
		}
		return "PASS: TestGood", nil
	}
	defer func() { executeShellCommand = originalExec }()

	// Execute
	cmd := NewGenerateTestsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{
		filePath,
		"--output", outputFile,
		"--auto-fix",
		"--max-retries", "1",
		"--run-cmd", "mock-test",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v\nOutput: %s", err, buf.String())
	}

	// Verify
	output := buf.String()
	if !strings.Contains(output, "Attempt 1/1") {
		t.Errorf("Should have attempted repair. Output: %s", output)
	}
	if !strings.Contains(output, "Tests passed!") {
		t.Errorf("Should report success. Output: %s", output)
	}

	// Verify file content matches the last good response
	content, _ := os.ReadFile(outputFile)
	if !strings.Contains(string(content), "func TestGood") {
		t.Errorf("File should contain fixed code. Got: %s", content)
	}
}
