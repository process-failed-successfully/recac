package main

import (
	"encoding/json"
	"os"
	"recac/internal/agent"
)

// loadAgentState loads the agent state from a JSON file.
func loadAgentState(filePath string) (*agent.State, error) {
	if filePath == "" {
		return nil, os.ErrNotExist // Or a more specific error
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var state agent.State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}
