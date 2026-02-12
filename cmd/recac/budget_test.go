package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBudgetCommand(t *testing.T) {
	// Setup temp dir for config and sessions
	tempDir := t.TempDir()
	sessionsDir := filepath.Join(tempDir, "sessions")
	err := os.Mkdir(sessionsDir, 0755)
	require.NoError(t, err)

	// Setup Config
	configFile := filepath.Join(tempDir, "config.yaml")
	viper.SetConfigFile(configFile)
	// Create empty config file
	err = os.WriteFile(configFile, []byte(""), 0644)
	require.NoError(t, err)

	// Reset viper after test
	defer viper.Reset()

	// Mock Session Manager
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		// Create archived dir as NewSessionManagerWithDir expects it or creates it
		return runner.NewSessionManagerWithDir(sessionsDir)
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// Create Mock Session Data
	session1State := &runner.SessionState{
		Name:           "test-session-1",
		Status:         "COMPLETED",
		StartTime:      time.Now(),
		AgentStateFile: filepath.Join(sessionsDir, "agent_state_1.json"),
	}
	agent1State := &agent.State{
		Model: "gpt-4",
		TokenUsage: agent.TokenUsage{
			TotalPromptTokens:   500,
			TotalResponseTokens: 500,
			TotalTokens:         1000,
		},
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
	// Clear and set
	for k := range agent.PricingTable {
		delete(agent.PricingTable, k)
	}
	agent.PricingTable["gpt-4"] = agent.PricePerMillionTokens{Prompt: 10.0, Completion: 30.0}
	// 500/1M * 10 + 500/1M * 30 = 0.005 + 0.015 = 0.02

	// Write session files
	sessionBytes, _ := json.Marshal(session1State)
	os.WriteFile(filepath.Join(sessionsDir, "test-session-1.json"), sessionBytes, 0644)
	agentBytes, _ := json.Marshal(agent1State)
	os.WriteFile(session1State.AgentStateFile, agentBytes, 0644)

	// --- Test 'budget set' ---
	// Reset viper config to be safe
	viper.Set("budget.limit", 0)

	// Capture stdout
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	// Execute Set
	// Use rootCmd to ensure proper command parsing
	rootCmd.SetArgs([]string{"budget", "set", "100"})
	err = rootCmd.Execute()
	require.NoError(t, err)

	// Verify config in memory
	val := viper.GetFloat64("budget.limit")
	assert.Equal(t, 100.0, val)

	// Verify output
	assert.Contains(t, buf.String(), "Budget set to $100.00")

	// --- Test 'budget status' ---
	buf.Reset()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	// Execute Status
	rootCmd.SetArgs([]string{"budget", "status"})
	err = rootCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Budget Status:")
	assert.Contains(t, output, "Limit:     $100.00")
	// Usage is 0.02
	assert.Contains(t, output, "Usage:     $0.02")
	assert.Contains(t, output, "Remaining: $99.98")

	// --- Test Warning ---
	viper.Set("budget.limit", 0.01)
	buf.Reset()
	rootCmd.SetArgs([]string{"budget", "status"})
	err = rootCmd.Execute()
	require.NoError(t, err)

	output = buf.String()
	assert.Contains(t, output, "WARNING: Budget exceeded")

	// Test error case: Budget not set
	viper.Set("budget.limit", 0)
	buf.Reset()
	rootCmd.SetArgs([]string{"budget", "status"})
	err = rootCmd.Execute()
	require.NoError(t, err)
	output = buf.String()
	assert.Contains(t, output, "Budget is not set")
}
