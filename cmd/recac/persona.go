package main

import (
	"fmt"
	"recac/internal/agent"
	"sort"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var personaCmd = &cobra.Command{
	Use:   "persona",
	Short: "Manage AI personas",
	Long:  `Create, list, and manage custom personas for the AI chat agent.`,
}

var personaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available personas",
	RunE: func(cmd *cobra.Command, args []string) error {
		all := loadAllPersonas()

		// Sort keys
		var keys []string
		for k := range all {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		fmt.Fprintln(cmd.OutOrStdout(), "Available Personas:")
		fmt.Fprintln(cmd.OutOrStdout(), "-------------------")
		for _, k := range keys {
			p := all[k]
			// Check if it is a default persona
			source := "(custom)"
			if _, ok := defaultPersonas[k]; ok {
				source = "(default)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-15s %-10s %s\n", k, source, p.Description)
		}
		return nil
	},
}

var personaAddCmd = &cobra.Command{
	Use:   "add [id]",
	Short: "Add a new custom persona",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var id, name, description, prompt string

		if len(args) > 0 {
			id = args[0]
		} else {
			if err := askOneFunc(&survey.Input{
				Message: "Persona ID (e.g., 'guru'):",
			}, &id); err != nil {
				return err
			}
		}

		if _, ok := defaultPersonas[id]; ok {
			return fmt.Errorf("persona ID '%s' is reserved by a default persona", id)
		}

		// Check if exists in custom
		path := getPersonasPath()
		custom, _ := agent.LoadPersonas(path)
		if _, ok := custom[id]; ok {
			fmt.Fprintf(cmd.OutOrStdout(), "Warning: Overwriting existing custom persona '%s'\n", id)
		}

		if err := askOneFunc(&survey.Input{
			Message: "Display Name (e.g., 'Go Guru'):",
		}, &name); err != nil {
			return err
		}

		if err := askOneFunc(&survey.Input{
			Message: "Description:",
		}, &description); err != nil {
			return err
		}

		if err := askOneFunc(&survey.Multiline{
			Message: "System Prompt:",
		}, &prompt); err != nil {
			return err
		}

		// Save
		custom[id] = agent.Persona{
			Name:         name,
			Description:  description,
			SystemPrompt: prompt,
		}

		if err := agent.SavePersonas(path, custom); err != nil {
			return fmt.Errorf("failed to save persona: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "✅ Persona '%s' added successfully.\n", id)
		return nil
	},
}

var personaRemoveCmd = &cobra.Command{
	Use:   "remove [id]",
	Aliases: []string{"rm", "delete"},
	Short: "Remove a custom persona",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		if _, ok := defaultPersonas[id]; ok {
			return fmt.Errorf("cannot remove default persona '%s'", id)
		}

		path := getPersonasPath()
		custom, err := agent.LoadPersonas(path)
		if err != nil {
			return err
		}

		if _, ok := custom[id]; !ok {
			return fmt.Errorf("persona '%s' not found", id)
		}

		delete(custom, id)

		if err := agent.SavePersonas(path, custom); err != nil {
			return fmt.Errorf("failed to save changes: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "🗑️  Persona '%s' removed.\n", id)
		return nil
	},
}

var personaShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show details of a persona",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		all := loadAllPersonas()

		p, ok := all[id]
		if !ok {
			return fmt.Errorf("persona '%s' not found", id)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "ID:           %s\n", id)
		fmt.Fprintf(cmd.OutOrStdout(), "Name:         %s\n", p.Name)
		fmt.Fprintf(cmd.OutOrStdout(), "Description:  %s\n", p.Description)
		fmt.Fprintln(cmd.OutOrStdout(), "System Prompt:")
		fmt.Fprintln(cmd.OutOrStdout(), "----------------------------------------")
		fmt.Fprintln(cmd.OutOrStdout(), p.SystemPrompt)
		fmt.Fprintln(cmd.OutOrStdout(), "----------------------------------------")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(personaCmd)
	personaCmd.AddCommand(personaListCmd)
	personaCmd.AddCommand(personaAddCmd)
	personaCmd.AddCommand(personaRemoveCmd)
	personaCmd.AddCommand(personaShowCmd)
}
