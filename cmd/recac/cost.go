package main

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"recac/internal/agent"
	"recac/internal/runner"
	"recac/internal/ui"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(costCmd)
	costCmd.Flags().Int("limit", 10, "Limit the number of sessions displayed in the 'Top Sessions by Cost' list")
	costCmd.Flags().Bool("watch", false, "Launch a real-time TUI to monitor session costs")
}

var costCmd = &cobra.Command{
	Use:   "cost",
	Short: "Analyze and display session costs",
	Long:  `Provides a detailed breakdown of costs associated with all sessions. Use the --watch flag for a live, real-time monitoring TUI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("could not create session manager: %w", err)
		}

		watch, _ := cmd.Flags().GetBool("watch")
		if watch {
			// Inject the agent state loader from this package into the ui package
			ui.LoadAgentState = loadAgentState
			// Start the TUI
			if err := ui.StartCostTUI(sm); err != nil {
				return fmt.Errorf("could not start cost TUI: %w", err)
			}
			return nil
		}

		sessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("could not list sessions: %w", err)
		}

		if len(sessions) == 0 {
			cmd.Println("No sessions found to analyze.")
			return nil
		}

		limit, _ := cmd.Flags().GetInt("limit")

		analysis, err := analyzeSessionCosts(sessions, limit)
		if err != nil {
			return fmt.Errorf("error analyzing session costs: %w", err)
		}

		displayCostAnalysis(cmd, analysis)

		return nil
	},
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
		// Skip sessions where agent state can't be loaded (e.g., still running, no agent yet)
		if err != nil {
			continue
		}

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

func displayCostAnalysis(cmd *cobra.Command, analysis *CostAnalysis) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)

	// --- Cost By Model ---
	fmt.Fprintln(w, "COST BY MODEL")
	fmt.Fprintln(w, "-------------")
	fmt.Fprintln(w, "MODEL\tCOST\tTOTAL TOKENS\tPROMPT TOKENS\tRESPONSE TOKENS")
	for _, model := range analysis.Models {
		fmt.Fprintf(w, "%s\t$%.4f\t%d\t%d\t%d\n",
			model.Name, model.TotalCost, model.TotalTokens, model.TotalPromptTokens, model.TotalResponseTokens)
	}
	fmt.Fprintln(w)

	// --- Top Sessions by Cost ---
	fmt.Fprintln(w, "TOP SESSIONS BY COST")
	fmt.Fprintln(w, "--------------------")
	fmt.Fprintln(w, "SESSION NAME\tMODEL\tCOST\tTOTAL TOKENS")
	for _, session := range analysis.TopSessionsByCost {
		fmt.Fprintf(w, "%s\t%s\t$%.6f\t%d\n",
			session.Name, session.Model, session.Cost, session.TotalTokens)
	}
	fmt.Fprintln(w)

	// --- Totals ---
	fmt.Fprintln(w, "TOTALS")
	fmt.Fprintln(w, "------")
	fmt.Fprintf(w, "Total Estimated Cost:\t$%.4f\n", analysis.TotalCost)
	fmt.Fprintf(w, "Total Tokens:\t%d\n", analysis.TotalTokens)

	w.Flush()
}
