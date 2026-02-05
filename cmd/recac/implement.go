package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"recac/internal/db"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var implementCmd = &cobra.Command{
	Use:   "implement [plan_file]",
	Short: "Iteratively implement features from a plan",
	Long: `Reads a feature list (default: feature_list.json) and sequentially implements each feature.
It starts a local coding session for each feature, tracking status in the plan file.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runImplement,
}

func init() {
	rootCmd.AddCommand(implementCmd)
	implementCmd.Flags().Bool("auto", false, "Skip confirmation prompts")
	implementCmd.Flags().String("from", "", "Resume execution from a specific Feature ID")
}

func runImplement(cmd *cobra.Command, args []string) error {
	planFile := "feature_list.json"
	if len(args) > 0 {
		planFile = args[0]
	}

	auto, _ := cmd.Flags().GetBool("auto")
	fromID, _ := cmd.Flags().GetString("from")

	// 1. Read Plan
	data, err := os.ReadFile(planFile)
	if err != nil {
		return fmt.Errorf("failed to read plan file %s: %w", planFile, err)
	}

	var featureList db.FeatureList
	if err := json.Unmarshal(data, &featureList); err != nil {
		return fmt.Errorf("failed to parse plan file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "📋 Implementation Plan: %s (%d features)\n", featureList.ProjectName, len(featureList.Features))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startFound := (fromID == "")

	for i, feature := range featureList.Features {
		// Check resume
		if !startFound {
			if feature.ID == fromID {
				startFound = true
			} else {
				continue
			}
		}

		if feature.Status == "Done" || feature.Status == "Completed" {
			fmt.Fprintf(cmd.OutOrStdout(), "✅ [%s] %s (Already Done)\n", feature.ID, feature.Description)
			continue
		}

		// Display
		fmt.Fprintf(cmd.OutOrStdout(), "\n👉 [%s] Category: %s | Priority: %s\n", feature.ID, feature.Category, feature.Priority)
		fmt.Fprintf(cmd.OutOrStdout(), "   Description: %s\n", feature.Description)
		if len(feature.Steps) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "   Steps:\n")
			for _, s := range feature.Steps {
				fmt.Fprintf(cmd.OutOrStdout(), "     - %s\n", s)
			}
		}

		// Confirm
		if !auto {
			fmt.Fprintf(cmd.OutOrStdout(), "   Start implementation? [Y/n/q] ")
			var response string
			fmt.Scanln(&response)
			response = strings.ToLower(response)
			if response == "q" {
				fmt.Println("Aborting.")
				return nil
			}
			if response == "n" {
				fmt.Println("Skipping.")
				continue
			}
		}

		// Start Session
		fmt.Fprintf(cmd.OutOrStdout(), "🚀 Starting session for %s...\n", feature.ID)

		// Create Description Prompt
		fullDesc := fmt.Sprintf("Feature ID: %s\nDescription: %s\n", feature.ID, feature.Description)
		if len(feature.Steps) > 0 {
			fullDesc += "\nImplementation Steps:\n"
			for _, s := range feature.Steps {
				fullDesc += fmt.Sprintf("- %s\n", s)
			}
		}

		// Configure Session
		cfg := SessionConfig{
			SessionName:       fmt.Sprintf("impl-%s", feature.ID),
			Goal:              fmt.Sprintf("Implement Feature %s: %s", feature.ID, feature.Description),
			ProjectPath:       ".", // Current directory
			ProjectName:       featureList.ProjectName,
			Description:       fullDesc,
			MaxIterations:     viper.GetInt("max_iterations"),
			ManagerFrequency:  viper.GetInt("manager_frequency"),
			TaskMaxIterations: viper.GetInt("task_max_iterations"),
			Provider:          viper.GetString("provider"),
			Model:             viper.GetString("model"),
			AllowDirty:        true,
			Stream:            true,
			Cleanup:           false,
		}

		// Execute
		if err := runWorkflowFunc(ctx, cfg); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "❌ Feature %s failed: %v\n", feature.ID, err)
			return fmt.Errorf("implementation of %s failed", feature.ID)
		}

		// Update Status
		fmt.Fprintf(cmd.OutOrStdout(), "✅ Feature %s completed.\n", feature.ID)
		featureList.Features[i].Status = "Done"
		featureList.Features[i].Passes = true

		// Save Progress
		newData, err := json.MarshalIndent(featureList, "", "  ")
		if err == nil {
			_ = os.WriteFile(planFile, newData, 0644)
		}
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\n🎉 All features implemented!")
	return nil
}
