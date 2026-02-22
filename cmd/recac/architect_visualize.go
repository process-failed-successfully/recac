package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/architecture"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var visualizeCmd = &cobra.Command{
	Use:   "visualize [path]",
	Short: "Visualize the system architecture",
	Long:  `Generates a Mermaid flowchart of the system architecture defined in architecture.yaml.`,
	RunE:  runArchitectVisualize,
}

func init() {
	architectCmd.AddCommand(visualizeCmd)
}

func runArchitectVisualize(cmd *cobra.Command, args []string) error {
	path := ".recac/architecture"
	if len(args) > 0 {
		path = args[0]
	}

	// If directory, append architecture.yaml
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to access %s: %w", path, err)
	}
	if info.IsDir() {
		path = filepath.Join(path, "architecture.yaml")
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	// Parse YAML
	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Generate Mermaid
	fmt.Fprintln(cmd.OutOrStdout(), generateMermaidSystemArch(&arch))
	return nil
}

func generateMermaidSystemArch(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Helper to sanitize IDs
	sanitize := func(id string) string {
		// Create a hash to ensure valid ID characters if name has spaces/special chars
		h := sha256.Sum256([]byte(id))
		return "N" + hex.EncodeToString(h[:4])
	}
	// Map to store original ID to sanitized ID
	idMap := make(map[string]string)
	for _, c := range arch.Components {
		idMap[c.ID] = sanitize(c.ID)
	}

	// Sort components for deterministic output
	sort.Slice(arch.Components, func(i, j int) bool {
		return arch.Components[i].ID < arch.Components[j].ID
	})

	for _, c := range arch.Components {
		nodeID := idMap[c.ID]
		label := strings.ReplaceAll(c.ID, "\"", "'")

		// Shapes based on type
		shapeStart := "["
		shapeEnd := "]"
		switch strings.ToLower(c.Type) {
		case "database":
			shapeStart = "[("
			shapeEnd = ")]"
		case "queue":
			shapeStart = "{{"
			shapeEnd = "}}"
		case "worker":
			shapeStart = "(["
			shapeEnd = "])"
		case "frontend":
			shapeStart = "(("
			shapeEnd = "))"
		}

		sb.WriteString(fmt.Sprintf("    %s%s\"%s\"%s\n", nodeID, shapeStart, label, shapeEnd))
	}

	sb.WriteString("\n")

	// Edges
	// We iterate components to find edges
	// 1. Consumes: Source -> Component
	for _, c := range arch.Components {
		targetID := idMap[c.ID]

		for _, input := range c.Consumes {
			if input.Source == "" {
				continue
			}
			sourceID, exists := idMap[input.Source]
			if !exists {
				// Source might be external or not defined in components list
				sourceID = sanitize(input.Source)
				sb.WriteString(fmt.Sprintf("    %s[\"%s (External)\"]\n", sourceID, input.Source))
				idMap[input.Source] = sourceID
			}

			// Label for edge? input.Type
			label := ""
			if input.Type != "" {
				label = fmt.Sprintf("|%s|", input.Type)
			}

			sb.WriteString(fmt.Sprintf("    %s -->%s %s\n", sourceID, label, targetID))
		}

		// 2. Produces: Component -> Target (if specified)
		for _, output := range c.Produces {
			if output.Target == "" {
				continue
			}
			targetNodeID, exists := idMap[output.Target]
			if !exists {
				targetNodeID = sanitize(output.Target)
				sb.WriteString(fmt.Sprintf("    %s[\"%s (External)\"]\n", targetNodeID, output.Target))
				idMap[output.Target] = targetNodeID
			}

			label := ""
			if output.Type != "" {
				label = fmt.Sprintf("|%s|", output.Type)
			} else if output.Event != "" {
				label = fmt.Sprintf("|Event: %s|", output.Event)
			}

			sb.WriteString(fmt.Sprintf("    %s -->%s %s\n", targetID, label, targetNodeID))
		}
	}

	return sb.String()
}
