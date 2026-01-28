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
	Short: "Visualize the feature plan as a Mermaid graph",
	Long:  `Generates a Mermaid flowchart from a feature list JSON file.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPlanVisualize,
}

func runPlanVisualize(cmd *cobra.Command, args []string) error {
	inputFile := "feature_list.json"
	if len(args) > 0 {
		inputFile = args[0]
	}

	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", inputFile, err)
	}

	var featureList db.FeatureList
	if err := json.Unmarshal(content, &featureList); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	mermaid := generateMermaidPlan(featureList)
	fmt.Fprintln(cmd.OutOrStdout(), mermaid)
	return nil
}

func generateMermaidPlan(list db.FeatureList) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Sort features by ID for deterministic output
	sort.Slice(list.Features, func(i, j int) bool {
		return list.Features[i].ID < list.Features[j].ID
	})

	// Map to check existence
	exists := make(map[string]bool)
	for _, f := range list.Features {
		exists[f.ID] = true
	}

	for _, f := range list.Features {
		// Node
		// ID[Summary]
		// Escape summary
		summary := strings.ReplaceAll(f.Description, "\"", "'")
		runes := []rune(summary)
		if len(runes) > 40 {
			summary = string(runes[:37]) + "..."
		}

		// Style based on Priority
		style := ""
		if strings.EqualFold(f.Priority, "MVP") {
			style = ":::mvp"
		} else if strings.EqualFold(f.Priority, "POC") {
			style = ":::poc"
		}

		// Sanitize ID
		safeID := strings.ReplaceAll(f.ID, "-", "_")
		safeID = strings.ReplaceAll(safeID, " ", "_")
		safeID = strings.ReplaceAll(safeID, ".", "_") // Sanitize dots too

		sb.WriteString(fmt.Sprintf("    %s[\"%s: %s\"]%s\n", safeID, f.ID, summary, style))

		// Edges
		for _, depID := range f.Dependencies.DependsOnIDs {
			if exists[depID] {
				safeDepID := strings.ReplaceAll(depID, "-", "_")
				safeDepID = strings.ReplaceAll(safeDepID, " ", "_")
				safeDepID = strings.ReplaceAll(safeDepID, ".", "_")
				sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeDepID, safeID))
			}
		}
	}

	// Styles
	sb.WriteString("\n    classDef mvp fill:#d4edda,stroke:#28a745,stroke-width:2px;\n")
	sb.WriteString("    classDef poc fill:#fff3cd,stroke:#ffc107,stroke-width:2px;\n")

	return sb.String()
}
