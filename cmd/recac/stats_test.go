package main

import (
	"bytes"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateStats(t *testing.T) {
	// Setup Session Manager
	sm := NewMockSessionManager()
	sm.Sessions["session1"] = &runner.SessionState{
		Name:           "session1",
		Status:         "completed",
		AgentStateFile: "state1.json",
	}
	sm.Sessions["session2"] = &runner.SessionState{
		Name:           "session2",
		Status:         "failed",
		AgentStateFile: "state2.json",
	}
	sm.Sessions["session3"] = &runner.SessionState{
		Name:           "session3",
		Status:         "completed",
		AgentStateFile: "state3.json",
	}

	// Mock loadAgentState
	originalLoad := loadAgentState
	defer func() { loadAgentState = originalLoad }()
	loadAgentState = func(filePath string) (*agent.State, error) {
		if filePath == "state1.json" {
			return &agent.State{
				TokenUsage: agent.TokenUsage{TotalTokens: 100, TotalPromptTokens: 80, TotalResponseTokens: 20},
				Model:      "gpt-4",
			}, nil
		}
		if filePath == "state2.json" {
			return &agent.State{
				TokenUsage: agent.TokenUsage{TotalTokens: 200, TotalPromptTokens: 150, TotalResponseTokens: 50},
				Model:      "gpt-3.5-turbo",
			}, nil
		}
		if filePath == "state3.json" {
			return &agent.State{
				TokenUsage: agent.TokenUsage{TotalTokens: 50, TotalPromptTokens: 40, TotalResponseTokens: 10},
				Model:      "gpt-4",
			}, nil
		}
		return nil, nil
	}

	stats, err := calculateStats(sm)
	assert.NoError(t, err)
	assert.Equal(t, 3, stats.TotalSessions)
	assert.Equal(t, 350, stats.TotalTokens)
	assert.Equal(t, 270, stats.TotalPromptTokens)
	assert.Equal(t, 80, stats.TotalResponseTokens)
	assert.Equal(t, 2, stats.StatusCounts["completed"])
	assert.Equal(t, 1, stats.StatusCounts["failed"])
	assert.Greater(t, stats.TotalCost, 0.0)
}

func TestDisplayStats(t *testing.T) {
	stats := &AggregateStats{
		TotalSessions:       10,
		TotalTokens:         1000,
		TotalPromptTokens:   800,
		TotalResponseTokens: 200,
		TotalCost:           1.2345,
		StatusCounts: map[string]int{
			"completed": 8,
			"failed":    2,
		},
	}

	var buf bytes.Buffer
	displayStats(&buf, stats)

	output := buf.String()
	assert.Contains(t, output, "AGGREGATE SESSION STATISTICS")
	assert.Contains(t, output, "Total Sessions:")
	assert.Contains(t, output, "10")
	assert.Contains(t, output, "Total Tokens:")
	assert.Contains(t, output, "1000")
	assert.Contains(t, output, "Total Estimated Cost:")
	assert.Contains(t, output, "$1.2345")
	assert.Contains(t, output, "completed:")
	assert.Contains(t, output, "8")
	assert.Contains(t, output, "failed:")
	assert.Contains(t, output, "2")
}

func TestStatsCmd_Run(t *testing.T) {
	// Mock Session Manager Factory
	sm := NewMockSessionManager()
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// Add a session
	sm.Sessions["test"] = &runner.SessionState{
		Name:           "test",
		Status:         "running",
		AgentStateFile: "test.json",
	}

	// Mock loadAgentState
	originalLoad := loadAgentState
	defer func() { loadAgentState = originalLoad }()
	loadAgentState = func(filePath string) (*agent.State, error) {
		return &agent.State{
			TokenUsage: agent.TokenUsage{TotalTokens: 10},
			Model:      "test-model",
		}, nil
	}

	output, err := executeCommand(rootCmd, "stats")
	assert.NoError(t, err)
	assert.Contains(t, output, "AGGREGATE SESSION STATISTICS")
	assert.Contains(t, output, "Total Sessions:")
	assert.Contains(t, output, "1")
	assert.Contains(t, output, "running:")
	assert.Contains(t, output, "1")
}
