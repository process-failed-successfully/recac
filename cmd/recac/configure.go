package main

import (
	"fmt"
	"os"
	"path/filepath"

	"recac/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Interactive configuration wizard",
	Long:  "Runs an interactive wizard to setup the configuration file for Recac.",
	RunE:  runConfigure,
}

func init() {
	rootCmd.AddCommand(configureCmd)
}

func runConfigure(cmd *cobra.Command, args []string) error {
	p := tea.NewProgram(ui.NewConfigWizardModel())
	m, err := p.Run()
	if err != nil {
		return fmt.Errorf("wizard failed: %w", err)
	}

	model, ok := m.(ui.ConfigWizardModel)
	if !ok {
		return fmt.Errorf("failed to retrieve wizard data")
	}

	if !model.Data.Confirmed {
		fmt.Println("Configuration cancelled.")
		return nil
	}

	// Update Viper
	viper.Set("provider", model.Data.Provider)
	if model.Data.APIKey != "" {
		viper.Set("api_key", model.Data.APIKey)
	}

	if model.Data.JiraEnabled {
		viper.Set("jira.url", model.Data.JiraURL)
		viper.Set("jira.username", model.Data.JiraEmail)
		viper.Set("jira.api_token", model.Data.JiraToken)
	}

	// Determine where to save
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		// If config file is not established, default to $HOME/.recac.yaml
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		configFile = filepath.Join(home, ".recac.yaml")
	}

	// Ensure the config type is set for marshaling
	viper.SetConfigType("yaml")

	if err := viper.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("failed to write config to %s: %w", configFile, err)
	}

	fmt.Printf("✅ Configuration saved to %s\n", configFile)
	return nil
}
