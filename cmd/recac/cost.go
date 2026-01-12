package main

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"recac/internal/agent"
	"recac/internal/runner"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(costCmd)
	costCmd.Flags().Int("limit", 10, "Limit the number of sessions displayed in the 'Top Sessions by Cost' list")
}

var costCmd = &cobra.Command{
	Use:   "cost",
	Short: "Analyze and display session costs",
	Long:  `Provides a detailed breakdown of costs associated with all sessions, grouped by model and sorted by expense.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("could not create session manager: %w", err)
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
