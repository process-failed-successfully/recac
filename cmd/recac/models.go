package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Manage AI models",
	Long:  `List available models from the configured provider and switch between them.`,
}

var modelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available models",
	RunE: func(cmd *cobra.Command, args []string) error {
		provider := viper.GetString("provider")
		model := viper.GetString("model")

		fmt.Fprintf(cmd.OutOrStdout(), "Fetching models for provider: %s...\n", provider)

		// Create agent to check for ModelLister capability
		ctx := context.Background()
		cwd, _ := os.Getwd()

		// Use factory or direct creation? Factory is safer but requires mocking in tests.
		// Since I am in main package, I can use agentClientFactory.
		ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-models")
		if err != nil {
			// Don't fail completely, maybe just fallback
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to initialize agent: %v. Using static list.\n", err)
		} else {
			if lister, ok := ag.(agent.ModelLister); ok {
				models, err := lister.ListModels(ctx)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to fetch models dynamically: %v\n", err)
					fmt.Fprintln(cmd.ErrOrStderr(), "Falling back to static list.")
				} else {
					printModelsList(cmd, provider, models)
					return nil
				}
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Provider does not support dynamic model listing. Showing known models.")
			}
		}

		// Fallback to static list (using existing loadAllModels from config_list.go)
		// loadAllModels returns map[string][]ModelItem
		allModels := loadAllModels()
		if items, ok := allModels[provider]; ok {
			var names []string
			for _, item := range items {
				// Format: value (description) if description differs
				if item.DescriptionDetails != "" && item.DescriptionDetails != item.Value {
					names = append(names, fmt.Sprintf("%s \t%s", item.Value, item.DescriptionDetails))
				} else {
					names = append(names, item.Value)
				}
			}
			printModelsListFallback(cmd, names)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "No known models for this provider.")
		}

		return nil
	},
}

var modelsUseCmd = &cobra.Command{
	Use:   "use [model_name]",
	Short: "Set the active model",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		newModel := args[0]
		viper.Set("model", newModel)

		if err := viper.WriteConfig(); err != nil {
			// If config file doesn't exist, we might want to try SafeWriteConfig
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				// Try to write to default location
				home, _ := os.UserHomeDir()
				configPath := filepath.Join(home, ".recac.yaml")
				if err := viper.WriteConfigAs(configPath); err != nil {
                     return fmt.Errorf("failed to write new config to %s: %w", configPath, err)
                }
			} else {
				return fmt.Errorf("failed to save config: %w", err)
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "✅ Switched to model: %s\n", newModel)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(modelsCmd)
	modelsCmd.AddCommand(modelsListCmd)
	modelsCmd.AddCommand(modelsUseCmd)
}

func printModelsList(cmd *cobra.Command, provider string, models []string) {
	sort.Strings(models)
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MODEL ID")
	fmt.Fprintln(w, "--------")
	for _, m := range models {
		fmt.Fprintln(w, m)
	}
	w.Flush()
}

func printModelsListFallback(cmd *cobra.Command, models []string) {
	sort.Strings(models)
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MODEL ID\tDESCRIPTION")
	fmt.Fprintln(w, "--------\t-----------")
	for _, m := range models {
		fmt.Fprintln(w, m)
	}
	w.Flush()
}
