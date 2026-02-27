package main

import (
	"fmt"
	"recac/internal/agent"
	"strings"

	"github.com/spf13/cobra"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage AI agent long-term memory",
	Long:  `Manage the long-term memory of the AI agent, stored in .agent_state.json.`,
}

var memoryAddCmd = &cobra.Command{
	Use:   "add [text]",
	Short: "Add a memory item",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text := strings.Join(args, " ")
		sm := agent.NewStateManager(".agent_state.json")

		if err := sm.AddMemory(text); err != nil {
			return fmt.Errorf("failed to add memory: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Memory added.")
		return nil
	},
}

var memoryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all memory items",
	RunE: func(cmd *cobra.Command, args []string) error {
		sm := agent.NewStateManager(".agent_state.json")
		state, err := sm.Load()
		if err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}

		if len(state.Memory) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No memory items found.")
			return nil
		}

		for i, m := range state.Memory {
			fmt.Fprintf(cmd.OutOrStdout(), "%d. %s\n", i+1, m)
		}
		return nil
	},
}

var memoryClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all memory",
	RunE: func(cmd *cobra.Command, args []string) error {
		sm := agent.NewStateManager(".agent_state.json")
		if err := sm.ClearMemory(); err != nil {
			return fmt.Errorf("failed to clear memory: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Memory cleared.")
		return nil
	},
}

func init() {
	if rootCmd != nil {
		rootCmd.AddCommand(memoryCmd)
	}
	memoryCmd.AddCommand(memoryAddCmd)
	memoryCmd.AddCommand(memoryListCmd)
	memoryCmd.AddCommand(memoryClearCmd)
}
