package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"recac/internal/db"

	"github.com/spf13/cobra"
)

var planVisualizeCmd = &cobra.Command{
	Use:   "visualize [feature_list.json]",
	Short: "Visualize the plan as a Mermaid graph",
	Long:  `Generates a Mermaid flowchart of the feature implementation plan.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPlanVisualize,
}

func init() {
	planCmd.AddCommand(planVisualizeCmd)
}

func runPlanVisualize(cmd *cobra.Command, args []string) error {
	inputFile := "feature_list.json"
	if len(args) > 0 {
		inputFile = args[0]
	}

	// Read File
	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", inputFile, err)
	}

	var featureList db.FeatureList
	if err := json.Unmarshal(content, &featureList); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Generate Mermaid
	fmt.Fprintln(cmd.OutOrStdout(), generateMermaidPlan(featureList))
	return nil
}

func generateMermaidPlan(fl db.FeatureList) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Sort features by ID for deterministic output
	features := fl.Features
	sort.Slice(features, func(i, j int) bool {
		return features[i].ID < features[j].ID
	})

	for _, f := range features {
		// Style based on Status
		style := ":::pending"
		switch strings.ToLower(f.Status) {
		case "done", "completed":
			style = ":::done"
		case "in_progress", "inprogress":
			style = ":::inprogress"
		case "failed":
			style = ":::failed"
		}

		safeID := sanitizeMermaidID(f.ID)

		// Truncate description for display
		desc := strings.ReplaceAll(f.Description, "\"", "'")
		desc = strings.ReplaceAll(desc, "\n", " ")
		runes := []rune(desc)
		if len(runes) > 30 {
			desc = string(runes[:27]) + "..."
		}

		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]%s\n", safeID, desc, style))

		for _, depID := range f.Dependencies.DependsOnIDs {
			safeDepID := sanitizeMermaidID(depID)
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeDepID, safeID))
		}
	}

	// Legend/Styles
	sb.WriteString("\n    classDef done fill:#90EE90,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef inprogress fill:#87CEEB,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef failed fill:#FF6347,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef pending fill:#D3D3D3,stroke:#333,stroke-width:1px,color:black;\n")

	return sb.String()
}
