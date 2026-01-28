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

func NewPlanVisualizeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "visualize [feature_list.json]",
		Short: "Visualize the plan as a dependency graph",
		Long:  `Generates a Mermaid flowchart of the feature implementation plan, showing dependencies and status.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runPlanVisualize,
	}
}

func runPlanVisualize(cmd *cobra.Command, args []string) error {
	inputFile := "feature_list.json"
	if len(args) > 0 {
		inputFile = args[0]
	}

	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file %s: %w", inputFile, err)
	}

	var fl db.FeatureList
	if err := json.Unmarshal(content, &fl); err != nil {
		return fmt.Errorf("failed to parse feature list: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), generateMermaidPlan(fl))
	return nil
}

func generateMermaidPlan(fl db.FeatureList) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Collect nodes to ensure deterministic output
	features := fl.Features
	sort.Slice(features, func(i, j int) bool {
		return features[i].ID < features[j].ID
	})

	for _, f := range features {
		// Style based on status
		style := ""
		switch strings.ToLower(f.Status) {
		case "completed", "done":
			style = ":::done"
		case "in_progress", "started":
			style = ":::inprogress"
		case "failed":
			style = ":::failed"
		case "poc":
			style = ":::poc"
		default: // Pending, Spec
			style = ":::pending"
		}

		safeID := SanitizeMermaidID(f.ID)

		// Truncate description for display
		desc := strings.ReplaceAll(f.Description, "\"", "'")
		desc = strings.ReplaceAll(desc, "\n", " ")
		runes := []rune(desc)
		if len(runes) > 40 {
			desc = string(runes[:37]) + "..."
		}

		// Node Label: "ID: Description"
		label := fmt.Sprintf("%s<br/>%s", f.ID, desc)

		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]%s\n", safeID, label, style))

		for _, depID := range f.Dependencies.DependsOnIDs {
			safeDepID := SanitizeMermaidID(depID)
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeDepID, safeID))
		}
	}

	// Styles
	sb.WriteString("\n    classDef done fill:#90EE90,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef inprogress fill:#87CEEB,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef failed fill:#FF6347,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef poc fill:#DDA0DD,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef pending fill:#D3D3D3,stroke:#333,stroke-width:1px,color:black;\n")

	return sb.String()
}
