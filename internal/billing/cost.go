package billing

import (
	"recac/internal/agent"
	"recac/internal/runner"
	"sort"
)

// CostAnalysis holds the aggregated cost data.
type CostAnalysis struct {
	TotalCost         float64
	TotalTokens       int
	Models            []*ModelCost
	TopSessionsByCost []*SessionCost
}

// ModelCost aggregates cost and token data for a specific model.
type ModelCost struct {
	Name                string
	TotalTokens         int
	TotalPromptTokens   int
	TotalResponseTokens int
	TotalCost           float64
}

// SessionCost holds cost data for a single session.
type SessionCost struct {
	Name        string
	Model       string
	Cost        float64
	TotalTokens int
}

// AnalyzeSessionCosts calculates cost based on session history.
func AnalyzeSessionCosts(sessions []*runner.SessionState, limit int) (*CostAnalysis, error) {
	modelCosts := make(map[string]*ModelCost)
	var sessionCosts []*SessionCost
	var totalCost float64
	var totalTokens int

	for _, session := range sessions {
		if session.AgentStateFile == "" {
			continue
		}

		// Use agent.LoadState directly as it's the standard way now
		// Note: agent.LoadState takes a file path.
		statePtr, err := agent.LoadState(session.AgentStateFile)
		if err != nil {
			// Skip sessions where agent state can't be loaded (e.g., deleted or still initializing)
			continue
		}
		agentState := *statePtr

		// Ensure model name is not empty
		if agentState.Model == "" {
			agentState.Model = "unknown"
		}

		cost := agent.CalculateCost(agentState.Model, agentState.TokenUsage)

		// Aggregate total stats
		totalCost += cost
		totalTokens += agentState.TokenUsage.TotalTokens

		// Aggregate by model
		if _, ok := modelCosts[agentState.Model]; !ok {
			modelCosts[agentState.Model] = &ModelCost{Name: agentState.Model}
		}
		model := modelCosts[agentState.Model]
		model.TotalTokens += agentState.TokenUsage.TotalTokens
		model.TotalPromptTokens += agentState.TokenUsage.TotalPromptTokens
		model.TotalResponseTokens += agentState.TokenUsage.TotalResponseTokens
		model.TotalCost += cost

		// Store session cost for sorting later
		sessionCosts = append(sessionCosts, &SessionCost{
			Name:        session.Name,
			Model:       agentState.Model,
			Cost:        cost,
			TotalTokens: agentState.TokenUsage.TotalTokens,
		})
	}

	// Sort models by cost (high to low)
	sortedModels := make([]*ModelCost, 0, len(modelCosts))
	for _, mc := range modelCosts {
		sortedModels = append(sortedModels, mc)
	}
	sort.Slice(sortedModels, func(i, j int) bool {
		return sortedModels[i].TotalCost > sortedModels[j].TotalCost
	})

	// Sort sessions by cost (high to low)
	sort.Slice(sessionCosts, func(i, j int) bool {
		return sessionCosts[i].Cost > sessionCosts[j].Cost
	})

	// Apply limit to top sessions
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
