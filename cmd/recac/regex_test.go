package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
)

// RegexTestMockAgent implements agent.Agent for testing
type RegexTestMockAgent struct {
	ResponseFunc func(prompt string) (string, error)
}

func (m *RegexTestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.ResponseFunc != nil {
		return m.ResponseFunc(prompt)
	}
	return "mock response", nil
}

func (m *RegexTestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil {
		onChunk(resp)
	}
	return resp, err
}

func TestRegexExplain(t *testing.T) {
	// Mock factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := &RegexTestMockAgent{
		ResponseFunc: func(prompt string) (string, error) {
			if strings.Contains(prompt, "Explain the following") {
				return "This regex matches generic text.", nil
			}
			return "", fmt.Errorf("unexpected prompt: %s", prompt)
		},
	}

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Reset flags
	regexExplain = "^[a-z]+$"
	regexMatch = nil
	regexNoMatch = nil
	regexLang = "go"

	cmd := regexCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := runRegex(cmd, []string{})
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Analyzing regex...")
	assert.Contains(t, buf.String(), "This regex matches generic text.")
}

func TestRegexGenerate(t *testing.T) {
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := &RegexTestMockAgent{
		ResponseFunc: func(prompt string) (string, error) {
			return "^[a-z]+$", nil
		},
	}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Reset flags
	regexExplain = ""
	regexMatch = []string{"abc"}
	regexNoMatch = []string{"123"}
	regexLang = "go"

	cmd := regexCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := runRegex(cmd, []string{"only lowercase letters"})
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Generating regex for: only lowercase letters")
	assert.Contains(t, buf.String(), "Matched 'abc'")
	assert.Contains(t, buf.String(), "Correctly rejected '123'")
	assert.Contains(t, buf.String(), "Regex verified successfully!")
	assert.Contains(t, buf.String(), "^[a-z]+$")
}

func TestRegexGenerate_Retry(t *testing.T) {
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	attempts := 0
	mockAgent := &RegexTestMockAgent{
		ResponseFunc: func(prompt string) (string, error) {
			attempts++
			if attempts == 1 {
				// First attempt: returns incorrect regex (matches numbers too)
				return "[a-z0-9]+", nil
			}
			// Second attempt: returns correct regex
			return "^[a-z]+$", nil
		},
	}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Reset flags
	regexExplain = ""
	regexMatch = []string{"abc"}
	regexNoMatch = []string{"123"}
	regexLang = "go"

	cmd := regexCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := runRegex(cmd, []string{"only lowercase letters"})
	assert.NoError(t, err)

	output := buf.String()
	// Should see failure first
	assert.Contains(t, output, "INCORRECTLY matched '123'")
	// Should see retry (Attempt 2 out of 3)
	assert.Contains(t, output, "Retrying (2/3)...")
	// Should see success eventually
	assert.Contains(t, output, "Regex verified successfully!")
	assert.Contains(t, output, "^[a-z]+$")
}

func TestRegexGenerate_CompileError(t *testing.T) {
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	attempts := 0
	mockAgent := &RegexTestMockAgent{
		ResponseFunc: func(prompt string) (string, error) {
			attempts++
			if attempts == 1 {
				// Invalid regex
				return "[a-z", nil
			}
			return "^[a-z]+$", nil
		},
	}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	regexExplain = ""
	regexMatch = nil
	regexNoMatch = nil
	regexLang = "go"

	cmd := regexCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer)) // Capture stderr to avoid pollution

	err := runRegex(cmd, []string{"lowercase"})
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Retrying (2/3)...") // It should retry (Attempt 2)
	assert.Contains(t, buf.String(), "^[a-z]+$")
}
