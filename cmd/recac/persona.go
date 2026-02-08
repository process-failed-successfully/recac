package main

import (
	"fmt"
	"recac/internal/agent"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

// Wrapper for survey.Ask to allow mocking if needed
var surveyAsk = survey.Ask
var surveyAskOneFunc = survey.AskOne

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
	Use:   "add [id]",
	Short: "Add a new custom persona",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get flags
		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("description")
		promptStr, _ := cmd.Flags().GetString("prompt")
		if promptStr == "" {
			promptStr, _ = cmd.Flags().GetString("system-prompt")
		}

		var id string
		if len(args) > 0 {
			id = args[0]
		}

		// Determine if we need interactive mode
		isInteractive := false
		if id == "" || name == "" || desc == "" || promptStr == "" {
			isInteractive = true
		}

		if isInteractive {
			var qs []*survey.Question
			if id == "" {
				qs = append(qs, &survey.Question{
					Name:     "id",
					Prompt:   &survey.Input{Message: "ID (short, lowercase, no spaces):"},
					Validate: survey.Required,
				})
			}
			if name == "" {
				qs = append(qs, &survey.Question{
					Name:     "name",
					Prompt:   &survey.Input{Message: "Display Name:"},
					Validate: survey.Required,
				})
			}
			if desc == "" {
				qs = append(qs, &survey.Question{
					Name:     "description",
					Prompt:   &survey.Input{Message: "Description:"},
					Validate: survey.Required,
				})
			}
			if promptStr == "" {
				qs = append(qs, &survey.Question{
					Name:     "system_prompt",
					Prompt:   &survey.Multiline{Message: "System Prompt:"},
					Validate: survey.Required,
				})
			}

			answers := make(map[string]interface{})
			if err := surveyAsk(qs, &answers); err != nil {
				return err
			}

			if v, ok := answers["id"].(string); ok {
				id = v
			}
			if v, ok := answers["name"].(string); ok {
				name = v
			}
			if v, ok := answers["description"].(string); ok {
				desc = v
			}
			if v, ok := answers["system_prompt"].(string); ok {
				promptStr = v
			}
		}

		id = strings.ToLower(strings.TrimSpace(id))

		pm := agent.NewPersonaManager()
		if err := pm.LoadPersonas(); err != nil {
			return err
		}

		if _, ok := pm.GetPersona(id); ok {
			// If interactive, ask to overwrite.
			if isInteractive {
				var overwrite bool
				prompt := &survey.Confirm{
					Message: fmt.Sprintf("Persona '%s' already exists. Overwrite?", id),
				}
				if err := surveyAskOneFunc(prompt, &overwrite); err != nil {
					return err
				}
				if !overwrite {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			} else {
				// Non-interactive: fail if exists (safety first)
				return fmt.Errorf("persona '%s' already exists (use interactive mode to overwrite)", id)
			}
		}

		pm.AddPersona(id, agent.Persona{
			Name:         name,
			Description:  desc,
			SystemPrompt: promptStr,
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
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			var confirm bool
			prompt := &survey.Confirm{
				Message: fmt.Sprintf("Are you sure you want to remove persona '%s'?", id),
			}
			if err := surveyAskOneFunc(prompt, &confirm); err != nil {
				return err
			}
			if !confirm {
				fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
				return nil
			}
		}

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

	// Add flags to add command
	personaAddCmd.Flags().StringP("name", "n", "", "Display name of the persona")
	personaAddCmd.Flags().StringP("description", "d", "", "Description of the persona")
	personaAddCmd.Flags().StringP("prompt", "p", "", "System prompt for the persona")
	personaAddCmd.Flags().String("system-prompt", "", "Alias for --prompt")

	// Add flags to remove command
	personaRemoveCmd.Flags().BoolP("force", "f", false, "Force removal without confirmation")
}
