package main

import (
	"encoding/json"
	"fmt"
	"os"

	"recac/internal/db"
	"recac/internal/runner"

	"github.com/spf13/cobra"
)

var planVisualizeCmd = &cobra.Command{
	Use:   "visualize [feature_list.json]",
	Short: "Visualize the planned feature dependencies",
	Long:  `Generates a Mermaid flowchart from the feature list JSON file.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  RunVisualize,
}

func RunVisualize(cmd *cobra.Command, args []string) error {
	file := "feature_list.json"
	if len(args) > 0 {
		file = args[0]
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", file, err)
	}

	var fl db.FeatureList
	if err := json.Unmarshal(content, &fl); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	g := runner.NewTaskGraph()
	if err := g.LoadFromFeatures(fl.Features); err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), GenerateMermaidGraph(g))
	return nil
}

func init() {
	planCmd.AddCommand(planVisualizeCmd)
}
