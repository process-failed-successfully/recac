package main

import (
	"context"
	"fmt"
	"os"
	"recac/internal/agent"
	"recac/internal/analysis"
	"testing"

	"github.com/stretchr/testify/assert"
)

// InfraMockAgent implements agent.Agent for testing
type InfraMockAgent struct {
	SendFunc func(ctx context.Context, prompt string) (string, error)
}

func (m *InfraMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, prompt)
	}
	return "", nil
}

func (m *InfraMockAgent) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestInfraCmd(t *testing.T) {
	// Backup original functions
	origAnalyze := analyzeDependenciesFunc
	origAgentFactory := agentClientFactory
	defer func() {
		analyzeDependenciesFunc = origAnalyze
		agentClientFactory = origAgentFactory
	}()

	// Mock AnalyzeDependencies
	analyzeDependenciesFunc = func(opts analysis.DependencyOptions) (analysis.DepMap, error) {
		return analysis.DepMap{
			"main": []string{"github.com/lib/pq", "github.com/go-redis/redis"},
		}, nil
	}

	// Mock Agent
	mockAgent := &InfraMockAgent{
		SendFunc: func(ctx context.Context, prompt string) (string, error) {
			// Verify prompt contains expected dependencies and target
			if !assert.Contains(t, prompt, "github.com/lib/pq") {
				return "", fmt.Errorf("prompt missing dependency pq")
			}
			if !assert.Contains(t, prompt, "github.com/go-redis/redis") {
				return "", fmt.Errorf("prompt missing dependency redis")
			}
			if !assert.Contains(t, prompt, "docker-compose") {
				return "", fmt.Errorf("prompt missing target docker-compose")
			}
			return "version: '3.8'\nservices:\n  db:\n    image: postgres", nil
		},
	}

	agentClientFactory = func(ctx context.Context, provider, model, cwd, purpose string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Create a temp file for output
	tmpFile, err := os.CreateTemp("", "infra_output.yaml")
	assert.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Set flags (global variables in main package)
	// We must reset them after test to avoid side effects if other tests run in parallel (though go test runs packages separately, files in same package share state).
	// To be safe, we invoke runInfra directly with the logic we want.
	// runInfra uses `infraOutput` and `infraTarget` global variables.
	origOutput := infraOutput
	origTarget := infraTarget
	defer func() {
		infraOutput = origOutput
		infraTarget = origTarget
	}()

	infraTarget = "docker-compose"
	infraOutput = tmpFile.Name()

	// Run Command
	// We pass the actual infraCmd, but its flags are parsed by Cobra before RunE.
	// Since we are calling runInfra directly, Cobra parsing hasn't updated the global variables from args.
	// But our implementation reads from the global variables `infraTarget` and `infraOutput`.
	// So setting them above is correct for this test.
	err = runInfra(infraCmd, []string{"."})
	assert.NoError(t, err)

	// Verify output file content
	content, err := os.ReadFile(tmpFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, "version: '3.8'\nservices:\n  db:\n    image: postgres", string(content))
}
