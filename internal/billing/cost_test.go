package billing

import (
	"fmt"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeSessionCosts(t *testing.T) {
	// Mock agent.LoadState
	originalLoadState := agent.LoadState
	defer func() { agent.LoadState = originalLoadState }()

	mockStates := map[string]*agent.State{
		"session1.json": {
			Model: "gpt-4",
			TokenUsage: agent.TokenUsage{
				TotalTokens:         1000,
				TotalPromptTokens:   500,
				TotalResponseTokens: 500,
			},
		},
		"session2.json": {
			Model: "gpt-3.5-turbo",
			TokenUsage: agent.TokenUsage{
				TotalTokens:         2000,
				TotalPromptTokens:   1000,
				TotalResponseTokens: 1000,
			},
		},
	}

	agent.LoadState = func(filePath string) (*agent.State, error) {
		if state, ok := mockStates[filePath]; ok {
			return state, nil
		}
		return nil, fmt.Errorf("file not found")
	}

	// Mock PricingTable - save original
	originalPricing := make(map[string]agent.PricePerMillionTokens)
	for k, v := range agent.PricingTable {
		originalPricing[k] = v
	}
	// Restore original after test
	defer func() {
		for k := range agent.PricingTable {
			delete(agent.PricingTable, k)
		}
		for k, v := range originalPricing {
			agent.PricingTable[k] = v
		}
	}()

	// Set mock pricing
	agent.PricingTable["gpt-4"] = agent.PricePerMillionTokens{Prompt: 10.0, Completion: 30.0}
	agent.PricingTable["gpt-3.5-turbo"] = agent.PricePerMillionTokens{Prompt: 1.0, Completion: 2.0}

	sessions := []*runner.SessionState{
		{Name: "session1", AgentStateFile: "session1.json"},
		{Name: "session2", AgentStateFile: "session2.json"},
		{Name: "session3", AgentStateFile: "missing.json"}, // Should be skipped
	}

	analysis, err := AnalyzeSessionCosts(sessions, 10)
	assert.NoError(t, err)
	assert.NotNil(t, analysis)

	// Calculate expected costs:
	// gpt-4: (500/1M * 10) + (500/1M * 30) = 0.005 + 0.015 = 0.02
	// gpt-3.5-turbo: (1000/1M * 1) + (1000/1M * 2) = 0.001 + 0.002 = 0.003
	// Total: 0.023

	// Use InDelta for float comparison
	if analysis.TotalCost < 0.0229 || analysis.TotalCost > 0.0231 {
		t.Errorf("Expected TotalCost around 0.023, got %f", analysis.TotalCost)
	}

	assert.Equal(t, 3000, analysis.TotalTokens)
	assert.Len(t, analysis.Models, 2)
	assert.Len(t, analysis.TopSessionsByCost, 2)

	// Verify sorting (cost descending)
	assert.Equal(t, "session1", analysis.TopSessionsByCost[0].Name)
	assert.Equal(t, "session2", analysis.TopSessionsByCost[1].Name)
}

func TestCheckBudget(t *testing.T) {
	// Setup mocks
	originalLoadState := agent.LoadState
	defer func() { agent.LoadState = originalLoadState }()

	mockStates := map[string]*agent.State{
		"session1.json": {
			Model: "gpt-4",
			TokenUsage: agent.TokenUsage{
				TotalTokens:         1000,
				TotalPromptTokens:   500,
				TotalResponseTokens: 500,
			},
		},
	}
	agent.LoadState = func(filePath string) (*agent.State, error) {
		if state, ok := mockStates[filePath]; ok {
			return state, nil
		}
		return nil, fmt.Errorf("file not found")
	}

	// Mock Pricing
	originalPricing := make(map[string]agent.PricePerMillionTokens)
	for k, v := range agent.PricingTable {
		originalPricing[k] = v
	}
	defer func() {
		for k := range agent.PricingTable {
			delete(agent.PricingTable, k)
		}
		for k, v := range originalPricing {
			agent.PricingTable[k] = v
		}
	}()
	agent.PricingTable["gpt-4"] = agent.PricePerMillionTokens{Prompt: 10.0, Completion: 30.0}

	sessions := []*runner.SessionState{
		{Name: "session1", AgentStateFile: "session1.json"},
	}

	// Test case: Budget not set (0)
	viper.Set("budget.limit", 0)
	_, _, _, err := CheckBudget(sessions)
	assert.ErrorIs(t, err, ErrBudgetNotSet)

	// Test case: Under budget
	viper.Set("budget.limit", 1.00)
	usage, limit, remaining, err := CheckBudget(sessions)
	assert.NoError(t, err)
	// Usage is 0.02
	assert.InDelta(t, 0.02, usage, 0.0001)
	assert.Equal(t, 1.00, limit)
	assert.InDelta(t, 0.98, remaining, 0.0001)

	// Test case: Over budget
	viper.Set("budget.limit", 0.01)
	usage, limit, remaining, err = CheckBudget(sessions)
	assert.NoError(t, err)
	assert.InDelta(t, 0.02, usage, 0.0001)
	assert.Equal(t, 0.01, limit)
	assert.Equal(t, 0.0, remaining)
}
