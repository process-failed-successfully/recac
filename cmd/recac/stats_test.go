package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"
)

func TestCalculateStats(t *testing.T) {
	tmpDir := t.TempDir()

	// --- Create Mock Agent State Files ---
	agentState1 := agent.State{
		Model: "gemini-1.5-pro-latest",
		TokenUsage: agent.TokenUsage{
			TotalPromptTokens:   100,
			TotalResponseTokens: 200,
			TotalTokens:         300,
		},
	}
	agentStateFile1 := filepath.Join(tmpDir, ".agent_state_1.json")
	agentData1, _ := json.Marshal(agentState1)
	os.WriteFile(agentStateFile1, agentData1, 0644)

	agentState2 := agent.State{
		Model: "claude-3-opus-20240229",
		TokenUsage: agent.TokenUsage{
			TotalPromptTokens:   50,
			TotalResponseTokens: 150,
			TotalTokens:         200,
		},
	}
	agentStateFile2 := filepath.Join(tmpDir, ".agent_state_2.json")
	agentData2, _ := json.Marshal(agentState2)
	os.WriteFile(agentStateFile2, agentData2, 0644)

	// --- Create Mock Session Manager ---
	mockSM := &MockSessionManager{
		Sessions: []*runner.SessionState{
			{
				Name:           "session1",
				Status:         "completed",
				AgentStateFile: agentStateFile1,
				StartTime:      time.Now(),
			},
			{
				Name:           "session2",
				Status:         "completed",
				AgentStateFile: agentStateFile2,
				StartTime:      time.Now(),
			},
			{
				Name:      "session3",
				Status:    "running",
				StartTime: time.Now(),
			},
		},
	}

	// Calculate stats
	stats, err := calculateStats(mockSM)
	if err != nil {
		t.Fatalf("calculateStats failed: %v", err)
	}

	// --- Assertions ---
	if stats.TotalSessions != 3 {
		t.Errorf("Expected 3 total sessions, but got %d", stats.TotalSessions)
	}
	if stats.TotalTokens != 500 {
		t.Errorf("Expected 500 total tokens, but got %d", stats.TotalTokens)
	}
	if stats.TotalPromptTokens != 150 {
		t.Errorf("Expected 150 prompt tokens, but got %d", stats.TotalPromptTokens)
	}
	if stats.TotalResponseTokens != 350 {
		t.Errorf("Expected 350 response tokens, but got %d", stats.TotalResponseTokens)
	}

	cost1 := agent.CalculateCost("gemini-1.5-pro-latest", agent.TokenUsage{TotalPromptTokens: 100, TotalResponseTokens: 200})
	cost2 := agent.CalculateCost("claude-3-opus-20240229", agent.TokenUsage{TotalPromptTokens: 50, TotalResponseTokens: 150})
	expectedCost := cost1 + cost2
	if stats.TotalCost != expectedCost {
		t.Errorf("Expected total cost of %.4f, but got %.4f", expectedCost, stats.TotalCost)
	}

	if stats.StatusCounts["completed"] != 2 {
		t.Errorf("Expected 2 completed sessions, but got %d", stats.StatusCounts["completed"])
	}
	if stats.StatusCounts["running"] != 1 {
		t.Errorf("Expected 1 running session, but got %d", stats.StatusCounts["running"])
	}
}
