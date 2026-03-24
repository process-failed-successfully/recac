package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"recac/internal/agent"
	"recac/internal/orchestrator"
)

type mockAIAgent struct {
	response string
	err      error
}

func (m *mockAIAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *mockAIAgent) SendStream(ctx context.Context, prompt string, streamFunc func(string)) (string, error) {
	return "", nil
}

func (m *mockAIAgent) Chat(ctx context.Context, messages []agent.Message) (string, error) {
	return "", nil
}

func TestOptimizePipelineJob_Success_Stdout(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	// Capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Override the newAgentFunc in orchestrator
	oldAgentFunc := orchestrator.GetNewAgentFunc()
	orchestrator.SetNewAgentFunc(func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		return &mockAIAgent{
			response: "optimized_content",
		}, nil
	})
	defer func() { orchestrator.SetNewAgentFunc(oldAgentFunc) }()

	tmpDir := t.TempDir()
	pPath := filepath.Join(tmpDir, "p1.yaml")
	err := os.WriteFile(pPath, []byte("original_content"), 0644)
	require.NoError(t, err)

	optimizePipelineJob(pPath, "-", "openai", "gpt-4o")

	out := buf.String()
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out, "Optimizing pipeline using AI")
	assert.Contains(t, out, "optimized_content")
}

func TestOptimizePipelineJob_Success_File(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldAgentFunc := orchestrator.GetNewAgentFunc()
	orchestrator.SetNewAgentFunc(func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		return &mockAIAgent{
			response: "optimized_content_file",
		}, nil
	})
	defer func() { orchestrator.SetNewAgentFunc(oldAgentFunc) }()

	tmpDir := t.TempDir()
	pPath := filepath.Join(tmpDir, "p1.yaml")
	err := os.WriteFile(pPath, []byte("original_content"), 0644)
	require.NoError(t, err)

	outPath := filepath.Join(tmpDir, "optimized.yaml")

	optimizePipelineJob(pPath, outPath, "openai", "gpt-4o")

	out := buf.String()
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out, "Optimizing pipeline using AI")
	assert.Contains(t, out, "Optimized pipeline successfully written to")

	fileContent, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Equal(t, "optimized_content_file", string(fileContent))
}

func TestOptimizePipelineJob_FileNotFound(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	optimizePipelineJob("non_existent.yaml", "-", "openai", "gpt-4o")

	out := buf.String()
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out, "Failed to read file")
}

func TestOptimizePipelineJob_AIError(t *testing.T) {
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldAgentFunc := orchestrator.GetNewAgentFunc()
	orchestrator.SetNewAgentFunc(func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		return &mockAIAgent{
			err: errors.New("ai error"),
		}, nil
	})
	defer func() { orchestrator.SetNewAgentFunc(oldAgentFunc) }()

	tmpDir := t.TempDir()
	pPath := filepath.Join(tmpDir, "p1.yaml")
	err := os.WriteFile(pPath, []byte("original_content"), 0644)
	require.NoError(t, err)

	optimizePipelineJob(pPath, "-", "openai", "gpt-4o")

	out := buf.String()
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out, "Failed to optimize pipeline")
}
