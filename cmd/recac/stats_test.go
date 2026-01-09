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
	sessionsDir := filepath.Join(tmpDir, "sessions")
	os.MkdirAll(sessionsDir, 0755)

	sm, err := runner.NewSessionManagerWithDir(sessionsDir)
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}
	sm.IsProcessRunning = func(pid int) bool {
		return true // Mock process as always running
	}

	// --- Create Mock Session 1 (with agent state) ---
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

	session1 := &runner.SessionState{
		Name:           "session1",
		Status:         "completed",
		AgentStateFile: agentStateFile1,
		StartTime:      time.Now(),
	}
	sm.SaveSession(session1)

	// --- Create Mock Session 2 (with agent state) ---
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

	session2 := &runner.SessionState{
		Name:           "session2",
		Status:         "completed",
		AgentStateFile: agentStateFile2,
		StartTime:      time.Now(),
	}
	sm.SaveSession(session2)

	// --- Create Mock Session 3 (without agent state) ---
	session3 := &runner.SessionState{
		Name:      "session3",
		Status:    "running",
		StartTime: time.Now(),
	}
	sm.SaveSession(session3)

	// Calculate stats
	stats, err := calculateStats(sm)
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
