package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var blueprintCmd = &cobra.Command{
	Use:   "blueprint [path]",
	Short: "Visualize the system architecture blueprint",
	Long: `Generates a Mermaid diagram from the architecture specification (architecture.yaml).
This allows you to visualize the components, their relationships, and data flow defined in the spec.

Example:
  recac blueprint
  recac blueprint .recac/architecture/architecture.yaml --output system_diagram.mmd`,
	RunE: runBlueprint,
}

func init() {
	rootCmd.AddCommand(blueprintCmd)
	blueprintCmd.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
	blueprintCmd.Flags().StringP("input", "i", ".recac/architecture/architecture.yaml", "Input architecture file")
}

func runBlueprint(cmd *cobra.Command, args []string) error {
	// Determine input path
	inputPath, _ := cmd.Flags().GetString("input")
	if len(args) > 0 {
		inputPath = args[0]
	}

	outputFile, _ := cmd.Flags().GetString("output")

	// Read File
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read architecture file '%s': %w", inputPath, err)
	}

	// Parse YAML
	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture YAML: %w", err)
	}

	// Generate Mermaid
	diagram := generateMermaidBlueprint(&arch)

	// Output
	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(diagram), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Blueprint diagram saved to %s\n", outputFile)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), diagram)
	}

	return nil
}

func generateMermaidBlueprint(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// 1. Nodes (Components)
	// Sort for deterministic output
	sort.Slice(arch.Components, func(i, j int) bool {
		return arch.Components[i].ID < arch.Components[j].ID
	})

	for _, c := range arch.Components {
		// Escape labels
		label := fmt.Sprintf("%s<br/>(%s)", c.ID, c.Type)
		if c.Description != "" {
			// Truncate description if too long?
			desc := c.Description
			if len(desc) > 30 {
				desc = desc[:27] + "..."
			}
			label += fmt.Sprintf("<br/>%s", desc)
		}

		// Sanitize ID for Mermaid (no spaces, dots etc)
		safeID := sanitizeBlueprintID(c.ID)
		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", safeID, label))
	}

	// 2. Edges
	// We use a map to deduplicate: "from|to" -> label/true
	edges := make(map[string]string)

	for _, c := range arch.Components {
		safeID := sanitizeBlueprintID(c.ID)

		// Consumes: Source -> Self
		for _, input := range c.Consumes {
			if input.Source == "" {
				continue
			}
			safeSource := sanitizeBlueprintID(input.Source)
			key := fmt.Sprintf("%s|%s", safeSource, safeID)

			label := input.Type
			if existing, ok := edges[key]; ok {
				if !strings.Contains(existing, label) {
					edges[key] = existing + ", " + label
				}
			} else {
				edges[key] = label
			}
		}

		// Produces: Self -> Target
		for _, output := range c.Produces {
			if output.Target == "" {
				continue
			}
			safeTarget := sanitizeBlueprintID(output.Target)
			key := fmt.Sprintf("%s|%s", safeID, safeTarget)

			label := output.Type
			if output.Event != "" {
				if label != "" {
					label += "/" + output.Event
				} else {
					label = output.Event
				}
			}

			if existing, ok := edges[key]; ok {
				if !strings.Contains(existing, label) {
					edges[key] = existing + ", " + label
				}
			} else {
				edges[key] = label
			}
		}
	}

	// Sort edges for deterministic output
	var edgeKeys []string
	for k := range edges {
		edgeKeys = append(edgeKeys, k)
	}
	sort.Strings(edgeKeys)

	for _, key := range edgeKeys {
		parts := strings.Split(key, "|")
		from := parts[0]
		to := parts[1]
		label := edges[key]

		if label != "" {
			sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", from, label, to))
		} else {
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", from, to))
		}
	}

	return sb.String()
}

func sanitizeBlueprintID(id string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, id)
}
