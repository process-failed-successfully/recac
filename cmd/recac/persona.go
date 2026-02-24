package main

import (
	"fmt"
	"recac/internal/agent"
	"strings"

	"github.com/spf13/cobra"
)

var (
	personaID           string
	personaName         string
	personaDesc         string
	personaSystemPrompt string
)

var personaCmd = &cobra.Command{
	Use:   "persona",
	Short: "Manage AI agent personas",
	Long:  `List, add, remove, and view AI agent personas used in chat and other commands.`,
}

var personaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available personas",
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := agent.NewPersonaManager()
		if err := pm.LoadPersonas(); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Available Personas:")
		fmt.Fprintln(cmd.OutOrStdout(), "-------------------")

		for _, id := range pm.ListSorted() {
			p, _ := pm.GetPersona(id)
			marker := ""
			if _, isDefault := agent.DefaultPersonas[id]; isDefault {
				marker = " (built-in)"
			} else {
				marker = " (custom)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "- %-15s %s%s\n", id, p.Name, marker)
		}
		return nil
	},
}

var personaShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show details of a persona",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		pm := agent.NewPersonaManager()
		if err := pm.LoadPersonas(); err != nil {
			return err
		}

		p, ok := pm.GetPersona(id)
		if !ok {
			return fmt.Errorf("persona '%s' not found", id)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Name:        %s\n", p.Name)
		fmt.Fprintf(cmd.OutOrStdout(), "ID:          %s\n", id)
		fmt.Fprintf(cmd.OutOrStdout(), "Description: %s\n", p.Description)
		fmt.Fprintf(cmd.OutOrStdout(), "System Prompt:\n%s\n", p.SystemPrompt)
		return nil
	},
}

var personaAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new custom persona",
	RunE: func(cmd *cobra.Command, args []string) error {
		if personaID == "" || personaName == "" || personaSystemPrompt == "" {
			return fmt.Errorf("id, name, and system-prompt are required")
		}

		id := strings.ToLower(strings.TrimSpace(personaID))

		pm := agent.NewPersonaManager()
		if err := pm.LoadPersonas(); err != nil {
			return err
		}

		pm.AddPersona(id, agent.Persona{
			Name:         personaName,
			Description:  personaDesc,
			SystemPrompt: personaSystemPrompt,
		})

		if err := pm.SavePersonas(); err != nil {
			return fmt.Errorf("failed to save personas: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "✅ Persona '%s' saved.\n", id)
		return nil
	},
}

var personaRemoveCmd = &cobra.Command{
	Use:     "remove [name]",
	Aliases: []string{"rm", "delete"},
	Short:   "Remove a custom persona",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		pm := agent.NewPersonaManager()
		if err := pm.LoadPersonas(); err != nil {
			return err
		}

		if err := pm.RemovePersona(id); err != nil {
			return err
		}

		if err := pm.SavePersonas(); err != nil {
			return fmt.Errorf("failed to save changes: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "🗑️ Persona '%s' removed.\n", id)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(personaCmd)
	personaCmd.AddCommand(personaListCmd)
	personaCmd.AddCommand(personaShowCmd)
	personaCmd.AddCommand(personaAddCmd)
	personaCmd.AddCommand(personaRemoveCmd)

	personaAddCmd.Flags().StringVar(&personaID, "id", "", "Short ID for the persona")
	personaAddCmd.Flags().StringVar(&personaName, "name", "", "Display name for the persona")
	personaAddCmd.Flags().StringVar(&personaDesc, "desc", "", "Description for the persona")
	personaAddCmd.Flags().StringVar(&personaSystemPrompt, "system-prompt", "", "System prompt for the persona")
}
