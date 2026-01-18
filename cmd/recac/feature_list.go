package main

import (
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/db"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var featureListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all features/tasks",
	Long:  `Displays a table of all features and their current status from feature_list.json.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fl, err := loadFeatures()
		if err != nil {
			return err
		}

		if len(fl.Features) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No features found.")
			return nil
		}

		printFeatureTable(cmd, fl.Features)
		return nil
	},
}

func init() {
	featureCmd.AddCommand(featureListCmd)
}

func loadFeatures() (*db.FeatureList, error) {
	// Try to read feature_list.json
	data, err := os.ReadFile("feature_list.json")
	if err != nil {
		if os.IsNotExist(err) {
			return &db.FeatureList{
				ProjectName: "current",
				Features:    []db.Feature{},
			}, nil
		}
		return nil, fmt.Errorf("failed to read feature_list.json: %w", err)
	}

	var fl db.FeatureList
	if err := json.Unmarshal(data, &fl); err != nil {
		return nil, fmt.Errorf("failed to parse feature_list.json: %w", err)
	}

	return &fl, nil
}

func saveFeatures(fl *db.FeatureList) error {
	out, err := json.MarshalIndent(fl, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal features: %w", err)
	}

	return os.WriteFile("feature_list.json", out, 0644)
}

func printFeatureTable(cmd *cobra.Command, features []db.Feature) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tPRIORITY\tDESCRIPTION")
	for _, f := range features {
		status := f.Status
		if f.Passes {
			status = "PASSING"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", f.ID, status, f.Priority, f.Description)
	}
	w.Flush()
}
