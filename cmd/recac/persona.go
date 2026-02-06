package main

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"recac/internal/agent"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var personaCmd = &cobra.Command{
	Use:   "persona",
	Short: "Manage AI personas",
	Long:  `List, switch, and create AI personas to change the agent's behavior.`,
}

var listPersonasCmd = &cobra.Command{
	Use:   "list",
	Short: "List available personas",
	Run: func(cmd *cobra.Command, args []string) {
		pm := agent.NewPersonaManager()
		personas := pm.ListPersonas()
		active := pm.GetActivePersona()

		// Sort by name
		sort.Slice(personas, func(i, j int) bool {
			return personas[i].Name < personas[j].Name
		})

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tDESCRIPTION\tACTIVE")
		for _, p := range personas {
			isActive := ""
			if p.Name == active.Name {
				isActive = "*"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", p.Name, p.Description, isActive)
		}
		w.Flush()
	},
}

var showPersonaCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show details of a persona",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		pm := agent.NewPersonaManager()
		p, found := pm.GetPersona(name)
		if !found {
			fmt.Printf("Persona '%s' not found.\n", name)
			return
		}

		fmt.Printf("Name: %s\n", p.Name)
		fmt.Printf("Description: %s\n", p.Description)
		fmt.Printf("System Prompt:\n%s\n", p.SystemPrompt)
	},
}

var usePersonaCmd = &cobra.Command{
	Use:   "use [name]",
	Short: "Set the active persona",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		pm := agent.NewPersonaManager()
		if err := pm.SetActivePersona(name); err != nil {
			return err
		}
		fmt.Printf("Active persona set to '%s'.\n", name)
		return nil
	},
}

var addPersonaCmd = &cobra.Command{
	Use:   "add",
	Short: "Create a new custom persona",
	RunE: func(cmd *cobra.Command, args []string) error {
		var qs = []*survey.Question{
			{
				Name: "name",
				Prompt: &survey.Input{
					Message: "Persona Name:",
				},
				Validate: survey.Required,
			},
			{
				Name: "description",
				Prompt: &survey.Input{
					Message: "Description:",
				},
			},
			{
				Name: "systemPrompt",
				Prompt: &survey.Multiline{
					Message: "System Prompt:",
				},
				Validate: survey.Required,
			},
		}

		answers := struct {
			Name         string
			Description  string
			SystemPrompt string
		}{}

		if err := survey.Ask(qs, &answers); err != nil {
			return err
		}

		p := agent.Persona{
			Name:         answers.Name,
			Description:  answers.Description,
			SystemPrompt: answers.SystemPrompt,
		}

		pm := agent.NewPersonaManager()
		if err := pm.SaveCustomPersona(p); err != nil {
			return fmt.Errorf("failed to save persona: %w", err)
		}

		fmt.Printf("Persona '%s' saved successfully.\n", p.Name)
		return nil
	},
}

var deletePersonaCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Delete a custom persona",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		pm := agent.NewPersonaManager()
		if err := pm.DeleteCustomPersona(name); err != nil {
			return err
		}
		fmt.Printf("Persona '%s' deleted.\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(personaCmd)
	personaCmd.AddCommand(listPersonasCmd)
	personaCmd.AddCommand(showPersonaCmd)
	personaCmd.AddCommand(usePersonaCmd)
	personaCmd.AddCommand(addPersonaCmd)
	personaCmd.AddCommand(deletePersonaCmd)
}
