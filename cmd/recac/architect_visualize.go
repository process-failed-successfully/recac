package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var visualizeCmd = &cobra.Command{
	Use:   "visualize [path]",
	Short: "Visualize the system architecture",
	Long:  "Generates a Mermaid graph of the system architecture defined in architecture.yaml.",
	RunE:  runVisualizeCmd,
}

func init() {
	architectCmd.AddCommand(visualizeCmd)
	visualizeCmd.Flags().StringP("out", "o", "", "Output file path (default: stdout)")
}

func runVisualizeCmd(cmd *cobra.Command, args []string) error {
	path := ".recac/architecture/architecture.yaml"
	if len(args) > 0 {
		path = args[0]
		// If directory provided, append filename
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			path = filepath.Join(path, "architecture.yaml")
		}
	}

	outPath, _ := cmd.Flags().GetString("out")

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read architecture file at %s: %w", path, err)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture file: %w", err)
	}

	mermaid := generateMermaidArchitecture(&arch)

	if outPath != "" {
		if err := os.WriteFile(outPath, []byte(mermaid), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Architecture diagram saved to %s\n", outPath)
	} else {
		fmt.Println(mermaid)
	}

	return nil
}

func generateMermaidArchitecture(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Helper to get safe ID
	getID := func(name string) string {
		h := sha256.New()
		h.Write([]byte(name))
		return "n" + hex.EncodeToString(h.Sum(nil))[:8]
	}

	// Map of component ID to safe node ID
	nodeIDs := make(map[string]string)

	// First pass: generate nodes
	for _, comp := range arch.Components {
		safeID := getID(comp.ID)
		nodeIDs[comp.ID] = safeID

		// Style based on type
		shapeStart := "["
		shapeEnd := "]"
		switch strings.ToLower(comp.Type) {
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

		label := comp.ID
		// Escape label quotes
		label = strings.ReplaceAll(label, "\"", "'")

		sb.WriteString(fmt.Sprintf("    %s%s\"%s\"%s\n", safeID, shapeStart, label, shapeEnd))
	}

	// Second pass: generate edges (Consumes)
	for _, comp := range arch.Components {
		targetNodeID := nodeIDs[comp.ID]

		for _, input := range comp.Consumes {
			sourceID := input.Source
			sourceNodeID, ok := nodeIDs[sourceID]

			// If source is external (not in our component list), create an external node
			if !ok {
				sourceNodeID = getID(sourceID)
				// Only add it once
				if _, exists := nodeIDs[sourceID]; !exists {
					nodeIDs[sourceID] = sourceNodeID
					sb.WriteString(fmt.Sprintf("    %s[\"%s (External)\"]:::external\n", sourceNodeID, sourceID))
				}
			}

			label := input.Type
			if label == "" {
				label = "uses"
			}
			label = strings.ReplaceAll(label, "\"", "'")

			sb.WriteString(fmt.Sprintf("    %s -- \"%s\" --> %s\n", sourceNodeID, label, targetNodeID))
		}
	}

	// Third pass: generate edges (Produces with explicit Target)
	for _, comp := range arch.Components {
		sourceNodeID := nodeIDs[comp.ID]

		for _, output := range comp.Produces {
			if output.Target == "" {
				continue // Implicit target (pub/sub or unknown), handled by consumer usually
			}

			targetID := output.Target
			targetNodeID, ok := nodeIDs[targetID]

			// If target is external
			if !ok {
				targetNodeID = getID(targetID)
				if _, exists := nodeIDs[targetID]; !exists {
					nodeIDs[targetID] = targetNodeID
					sb.WriteString(fmt.Sprintf("    %s[\"%s (External)\"]:::external\n", targetNodeID, targetID))
				}
			}

			label := output.Event
			if label == "" {
				label = output.Type
			}
			if label == "" {
				label = "sends"
			}
			label = strings.ReplaceAll(label, "\"", "'")

			sb.WriteString(fmt.Sprintf("    %s -- \"%s\" --> %s\n", sourceNodeID, label, targetNodeID))
		}
	}

	// Styles
	sb.WriteString("\n    classDef external fill:#f9f,stroke:#333,stroke-width:2px,stroke-dasharray: 5 5;\n")

	return sb.String()
}
