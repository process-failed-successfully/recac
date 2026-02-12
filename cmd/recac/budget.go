package main

import (
	"errors"
	"fmt"
	"recac/internal/billing"
	"strconv"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(budgetCmd)
	budgetCmd.AddCommand(budgetSetCmd)
	budgetCmd.AddCommand(budgetStatusCmd)
}

var budgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Manage project budget",
	Long:  `Set and monitor budget limits for your autonomous coding sessions.`,
}

var budgetSetCmd = &cobra.Command{
	Use:   "set [amount]",
	Short: "Set a budget limit in USD",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		amount, err := strconv.ParseFloat(args[0], 64)
		if err != nil {
			return fmt.Errorf("invalid amount: %w", err)
		}

		if err := billing.SetBudget(amount); err != nil {
			return fmt.Errorf("failed to set budget: %w", err)
		}

		cmd.Printf("Budget set to $%.2f\n", amount)
		return nil
	},
}

var budgetStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check current budget status",
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("could not create session manager: %w", err)
		}

		sessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("could not list sessions: %w", err)
		}

		usage, limit, remaining, err := billing.CheckBudget(sessions)
		if err != nil {
			// Special handling if budget is not set
			if errors.Is(err, billing.ErrBudgetNotSet) {
				cmd.Println("Budget is not set. Use 'recac budget set <amount>' to set a limit.")
				return nil
			}
			return err
		}

		percent := (usage / limit) * 100

		// Simple progress bar
		barWidth := 20
		filled := int((percent / 100) * float64(barWidth))
		if filled > barWidth {
			filled = barWidth
		}
		bar := "["
		for i := 0; i < filled; i++ {
			bar += "#"
		}
		for i := filled; i < barWidth; i++ {
			bar += "-"
		}
		bar += "]"

		cmd.Printf("Budget Status:\n")
		cmd.Printf("--------------\n")
		cmd.Printf("Limit:     $%.2f\n", limit)
		cmd.Printf("Usage:     $%.2f\n", usage)
		cmd.Printf("Remaining: $%.2f\n", remaining)
		cmd.Printf("Progress:  %s %.1f%%\n", bar, percent)

		if usage > limit {
			cmd.Printf("\nWARNING: Budget exceeded by $%.2f!\n", usage-limit)
		}

		return nil
	},
}
