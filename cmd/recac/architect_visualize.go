package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var architectVisualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Visualize the generated architecture",
	Long:  `Generates a Mermaid flowchart of the system architecture defined in architecture.yaml.`,
	RunE:  runArchitectVisualize,
}

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().String("dir", ".recac/architecture", "Directory containing architecture.yaml")
}

func runArchitectVisualize(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	archPath := filepath.Join(dir, "architecture.yaml")

	data, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", archPath, err)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture.yaml: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), generateMermaidArchitecture(&arch))
	return nil
}

func generateMermaidArchitecture(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Sort components for deterministic output
	sort.Slice(arch.Components, func(i, j int) bool {
		return arch.Components[i].ID < arch.Components[j].ID
	})

	seenEdges := make(map[string]bool)

	for _, comp := range arch.Components {
		// Use local safeID to ensure robustness against quotes/special chars
		safeID := safeMermaidID(comp.ID)

		label := comp.ID
		if comp.Type != "" {
			label = fmt.Sprintf("%s\\n(%s)", comp.ID, comp.Type)
		}
		// Escape quotes for Mermaid label
		label = strings.ReplaceAll(label, "\"", "'")

		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", safeID, label))

		// Apply basic styling based on type
		switch strings.ToLower(comp.Type) {
		case "database", "db", "storage":
			sb.WriteString(fmt.Sprintf("    class %s database;\n", safeID))
		case "worker", "job":
			sb.WriteString(fmt.Sprintf("    class %s worker;\n", safeID))
		case "service", "api":
			sb.WriteString(fmt.Sprintf("    class %s service;\n", safeID))
		}

		// Draw Edges based on Consumes (Input)
		// Source -> Component
		for _, input := range comp.Consumes {
			if input.Source != "" {
				safeSource := safeMermaidID(input.Source)
				// Label with Type
				edgeLabel := input.Type
				if edgeLabel == "" {
					edgeLabel = input.Schema
				}

				key := fmt.Sprintf("%s|%s|%s", safeSource, edgeLabel, safeID)
				if seenEdges[key] {
					continue
				}
				seenEdges[key] = true

				if edgeLabel != "" {
					sb.WriteString(fmt.Sprintf("    %s -- %s --> %s\n", safeSource, edgeLabel, safeID))
				} else {
					sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeSource, safeID))
				}
			}
		}

		// Draw Edges based on Produces (Output)
		// Component -> Target
		for _, output := range comp.Produces {
			if output.Target != "" {
				safeTarget := safeMermaidID(output.Target)
				// Label
				edgeLabel := output.Event
				if edgeLabel == "" {
					edgeLabel = output.Type
				}

				key := fmt.Sprintf("%s|%s|%s", safeID, edgeLabel, safeTarget)
				if seenEdges[key] {
					continue
				}
				seenEdges[key] = true

				if edgeLabel != "" {
					sb.WriteString(fmt.Sprintf("    %s -- %s --> %s\n", safeID, edgeLabel, safeTarget))
				} else {
					sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeID, safeTarget))
				}
			}
		}
	}

	// Add styling definitions
	sb.WriteString("\n    classDef service fill:#f9f,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef database fill:#ff9,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef worker fill:#9ff,stroke:#333,stroke-width:2px,color:black;\n")

	return sb.String()
}

func safeMermaidID(id string) string {
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, ".", "_")
	id = strings.ReplaceAll(id, "\"", "")
	id = strings.ReplaceAll(id, "'", "")
	return id
}
