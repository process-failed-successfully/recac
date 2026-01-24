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

// CronTestMockAgent implements agent.Agent for testing
type CronTestMockAgent struct {
	ResponseFunc func(prompt string) (string, error)
}

func (m *CronTestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.ResponseFunc != nil {
		return m.ResponseFunc(prompt)
	}
	return "mock response", nil
}

func (m *CronTestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil {
		onChunk(resp)
	}
	return resp, err
}

func TestCronGenerate(t *testing.T) {
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := &CronTestMockAgent{
		ResponseFunc: func(prompt string) (string, error) {
			// Simulate generating "every 5 minutes"
			return "*/5 * * * *", nil
		},
	}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Reset flags
	cronExplain = ""
	cronNext = 3

	cmd := cronCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := runCron(cmd, []string{"every 5 minutes"})
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Generating cron for: every 5 minutes")
	assert.Contains(t, output, "Result: */5 * * * *")
	assert.Contains(t, output, "Next scheduled runs:")
	// Ensure we have 3 lines of next runs
	// Header + output lines.
	// Approximate check: should contain "1." and "2." and "3."
	// Note: tabwriter replaces \t with spaces, so we just check for the number and dot.
	assert.Contains(t, output, "1.")
	assert.Contains(t, output, "2.")
	assert.Contains(t, output, "3.")
}

func TestCronExplain(t *testing.T) {
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := &CronTestMockAgent{
		ResponseFunc: func(prompt string) (string, error) {
			if strings.Contains(prompt, "Explain the following") {
				return "Runs every 5 minutes.", nil
			}
			return "", fmt.Errorf("unexpected prompt: %s", prompt)
		},
	}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	cronExplain = "*/5 * * * *"
	cronNext = 1

	cmd := cronCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := runCron(cmd, []string{})
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Analyzing cron expression...")
	assert.Contains(t, output, "Runs every 5 minutes.")
	assert.Contains(t, output, "Next scheduled runs:")
}

func TestCronInvalid(t *testing.T) {
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := &CronTestMockAgent{
		ResponseFunc: func(prompt string) (string, error) {
			return "invalid-cron", nil
		},
	}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	cronExplain = ""
	cronNext = 5

	cmd := cronCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	errBuf := new(bytes.Buffer)
	cmd.SetErr(errBuf)

	err := runCron(cmd, []string{"something weird"})
	assert.NoError(t, err) // It shouldn't return error, just print warning

	output := buf.String()
	errOutput := errBuf.String()

	assert.Contains(t, output, "Result: invalid-cron")
	assert.Contains(t, errOutput, "Could not parse cron expression")
}
