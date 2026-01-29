package main

import (
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/db"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var visualizeCmd = &cobra.Command{
	Use:   "visualize [feature_list.json]",
	Short: "Visualize the feature implementation plan",
	Long:  `Generates a Mermaid dependency graph from the feature list JSON.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPlanVisualize,
}

func init() {
	planCmd.AddCommand(visualizeCmd)
}

func runPlanVisualize(cmd *cobra.Command, args []string) error {
	file := "feature_list.json"
	if len(args) > 0 {
		file = args[0]
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read feature list from %s: %w", file, err)
	}

	var featureList db.FeatureList
	if err := json.Unmarshal(content, &featureList); err != nil {
		return fmt.Errorf("failed to parse feature list: %w", err)
	}

	// Generate Mermaid
	fmt.Fprintln(cmd.OutOrStdout(), generatePlanMermaid(&featureList))
	return nil
}

func generatePlanMermaid(fl *db.FeatureList) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Sort features by ID for determinism
	features := fl.Features
	sort.Slice(features, func(i, j int) bool {
		return features[i].ID < features[j].ID
	})

	for _, f := range features {
		safeID := SanitizeMermaidID(f.ID)

		// Create a label with ID and Category
		label := fmt.Sprintf("<b>%s</b><br/>%s", f.ID, f.Category)
		if len(f.Description) > 0 {
			// truncate description
			desc := strings.ReplaceAll(f.Description, "\"", "'")
			if len(desc) > 40 {
				desc = desc[:37] + "..."
			}
			label += fmt.Sprintf("<br/><i>%s</i>", desc)
		}

		// Style based on priority
		style := ""
		if strings.EqualFold(f.Priority, "POC") {
			style = ":::poc"
		} else if strings.EqualFold(f.Priority, "MVP") {
			style = ":::mvp"
		} else {
			style = ":::prod"
		}

		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]%s\n", safeID, label, style))

		// Dependencies
		for _, dep := range f.Dependencies.DependsOnIDs {
			safeDep := SanitizeMermaidID(dep)
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeDep, safeID))
		}
	}

	// Styles
	sb.WriteString("\n    classDef poc fill:#e1f5fe,stroke:#01579b,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef mvp fill:#fff9c4,stroke:#fbc02d,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef prod fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px,color:black;\n")

	return sb.String()
}
