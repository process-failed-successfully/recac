package main

import (
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/agent"
)

// loadAgentState reads and decodes the agent state file.
func loadAgentState(filePath string) (agent.State, error) {
	var state agent.State
	data, err := os.ReadFile(filePath)
	if err != nil {
		return state, fmt.Errorf("failed to read agent state file: %w", err)
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("failed to unmarshal agent state: %w", err)
	}
	return state, nil
}
