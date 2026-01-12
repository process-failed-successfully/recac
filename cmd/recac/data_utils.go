package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"recac/internal/agent"
	"recac/internal/runner"
)

// --- Data Loading and Analysis Utilities ---

// loadAgentState is a helper to read and parse an agent state file.
func loadAgentState(filePath string) (*agent.State, error) {
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

// AggregateStats holds the calculated statistics
type AggregateStats struct {
	TotalSessions       int
	TotalTokens         int
	TotalPromptTokens   int
	TotalResponseTokens int
	TotalCost           float64
	StatusCounts        map[string]int
}

// calculateStats calculates aggregate statistics from a list of sessions.
func calculateStats(sessions []*runner.SessionState) (*AggregateStats, error) {
	stats := &AggregateStats{
		StatusCounts: make(map[string]int),
	}

	for _, session := range sessions {
		stats.TotalSessions++
		stats.StatusCounts[session.Status]++

		if session.AgentStateFile == "" {
			continue
		}

		agentState, err := loadAgentState(session.AgentStateFile)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			continue
		}

		stats.TotalTokens += agentState.TokenUsage.TotalTokens
		stats.TotalPromptTokens += agentState.TokenUsage.TotalPromptTokens
		stats.TotalResponseTokens += agentState.TokenUsage.TotalResponseTokens
		stats.TotalCost += agent.CalculateCost(agentState.Model, agentState.TokenUsage)
	}

	return stats, nil
}

// CostAnalysis holds the aggregated cost data.
type CostAnalysis struct {
	TotalCost         float64
	TotalTokens       int
	Models            []*ModelCost
	TopSessionsByCost []*SessionCost
}

// ModelCost aggregates cost and token data for a specific model.
type ModelCost struct {
	Name              string
	TotalTokens       int
	TotalPromptTokens int
	TotalResponseTokens int
	TotalCost         float64
}

// SessionCost holds cost data for a single session.
type SessionCost struct {
	Name      string
	Model     string
	Cost      float64
	TotalTokens int
}

// analyzeSessionCosts provides a detailed breakdown of costs.
func analyzeSessionCosts(sessions []*runner.SessionState, limit int) (*CostAnalysis, error) {
	modelCosts := make(map[string]*ModelCost)
	var sessionCosts []*SessionCost
	var totalCost float64
	var totalTokens int

	for _, session := range sessions {
		if session.AgentStateFile == "" {
			continue
		}

		agentState, err := loadAgentState(session.AgentStateFile)
		if err != nil {
			continue
		}

		if agentState.Model == "" {
			agentState.Model = "unknown"
		}

		cost := agent.CalculateCost(agentState.Model, agentState.TokenUsage)
		totalCost += cost
		totalTokens += agentState.TokenUsage.TotalTokens

		if _, ok := modelCosts[agentState.Model]; !ok {
			modelCosts[agentState.Model] = &ModelCost{Name: agentState.Model}
		}
		model := modelCosts[agentState.Model]
		model.TotalTokens += agentState.TokenUsage.TotalTokens
		model.TotalPromptTokens += agentState.TokenUsage.TotalPromptTokens
		model.TotalResponseTokens += agentState.TokenUsage.TotalResponseTokens
		model.TotalCost += cost

		sessionCosts = append(sessionCosts, &SessionCost{
			Name:      session.Name,
			Model:     agentState.Model,
			Cost:      cost,
			TotalTokens: agentState.TokenUsage.TotalTokens,
		})
	}

	sortedModels := make([]*ModelCost, 0, len(modelCosts))
	for _, mc := range modelCosts {
		sortedModels = append(sortedModels, mc)
	}
	sort.Slice(sortedModels, func(i, j int) bool {
		return sortedModels[i].TotalCost > sortedModels[j].TotalCost
	})

	sort.Slice(sessionCosts, func(i, j int) bool {
		return sessionCosts[i].Cost > sessionCosts[j].Cost
	})

	if limit > 0 && len(sessionCosts) > limit {
		sessionCosts = sessionCosts[:limit]
	}

	return &CostAnalysis{
		TotalCost:         totalCost,
		TotalTokens:       totalTokens,
		Models:            sortedModels,
		TopSessionsByCost: sessionCosts,
	}, nil
}
