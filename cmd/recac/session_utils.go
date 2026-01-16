package main

import (
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/agent"
)

// loadAgentState is a helper to read and parse an agent state file.
var loadAgentState = func(filePath string) (*agent.State, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path is empty")
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
