package main

import (
	"fmt"
	"io"
	"os"
	"recac/internal/agent"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
	Use:   "add",
	Short: "Add a new custom persona",
	RunE: func(cmd *cobra.Command, args []string) error {
		var qs = []*survey.Question{
			{
				Name: "id",
				Prompt: &survey.Input{
					Message: "ID (short, lowercase, no spaces):",
				},
				Validate: survey.Required,
			},
			{
				Name: "name",
				Prompt: &survey.Input{
					Message: "Display Name:",
				},
				Validate: survey.Required,
			},
			{
				Name: "description",
				Prompt: &survey.Input{
					Message: "Description:",
				},
				Validate: survey.Required,
			},
			{
				Name: "system_prompt",
				Prompt: &survey.Multiline{
					Message: "System Prompt:",
				},
				Validate: survey.Required,
			},
		}

		answers := struct {
			ID           string `survey:"id"`
			Name         string `survey:"name"`
			Description  string `survey:"description"`
			SystemPrompt string `survey:"system_prompt"`
		}{}

		if err := surveyAsk(qs, &answers); err != nil {
			return err
		}

		id := strings.ToLower(strings.TrimSpace(answers.ID))

		pm := agent.NewPersonaManager()
		if err := pm.LoadPersonas(); err != nil {
			return err
		}

		if _, ok := pm.GetPersona(id); ok {
			// Ask to overwrite?
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
		}

		pm.AddPersona(id, agent.Persona{
			Name:         answers.Name,
			Description:  answers.Description,
			SystemPrompt: answers.SystemPrompt,
		})

		if err := pm.SavePersonas(); err != nil {
			return fmt.Errorf("failed to save personas: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "✅ Persona '%s' saved.\n", id)
		return nil
	},
}

var personaRemoveCmd = &cobra.Command{
	Use:   "remove [name]",
	Aliases: []string{"rm", "delete"},
	Short: "Remove a custom persona",
	Args:  cobra.ExactArgs(1),
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

var personaExportCmd = &cobra.Command{
	Use:   "export [id]",
	Short: "Export personas to YAML",
	Long:  "Export a specific persona by ID, or all custom personas if no ID is specified.",
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := agent.NewPersonaManager()
		if err := pm.LoadPersonas(); err != nil {
			return err
		}

		toExport := make(map[string]agent.Persona)

		if len(args) > 0 {
			id := args[0]
			p, ok := pm.GetPersona(id)
			if !ok {
				return fmt.Errorf("persona '%s' not found", id)
			}
			toExport[id] = p
		} else {
			// Export all custom personas
			all := pm.ListPersonas()
			for id, p := range all {
				isBuiltIn := false
				if def, ok := agent.DefaultPersonas[id]; ok {
					if def == p {
						isBuiltIn = true
					}
				}

				if !isBuiltIn {
					toExport[id] = p
				}
			}
		}

		data, err := yaml.Marshal(toExport)
		if err != nil {
			return fmt.Errorf("failed to marshal personas: %w", err)
		}

		fmt.Fprint(cmd.OutOrStdout(), string(data))
		return nil
	},
}

var personaImportCmd = &cobra.Command{
	Use:   "import [file]",
	Short: "Import personas from a YAML file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		var data []byte
		var err error

		if filePath == "-" {
			data, err = io.ReadAll(cmd.InOrStdin())
		} else {
			data, err = os.ReadFile(filePath)
		}
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		var imported map[string]agent.Persona
		if err := yaml.Unmarshal(data, &imported); err != nil {
			return fmt.Errorf("failed to parse YAML: %w", err)
		}

		pm := agent.NewPersonaManager()
		if err := pm.LoadPersonas(); err != nil {
			return err
		}

		count := 0
		for id, p := range imported {
			pm.AddPersona(id, p)
			count++
		}

		if err := pm.SavePersonas(); err != nil {
			return fmt.Errorf("failed to save personas: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully imported %d personas.\n", count)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(personaCmd)
	personaCmd.AddCommand(personaListCmd)
	personaCmd.AddCommand(personaShowCmd)
	personaCmd.AddCommand(personaAddCmd)
	personaCmd.AddCommand(personaRemoveCmd)
	personaCmd.AddCommand(personaExportCmd)
	personaCmd.AddCommand(personaImportCmd)
}
