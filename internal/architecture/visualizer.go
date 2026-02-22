package architecture

import (
	"fmt"
	"sort"
	"strings"
)

// GenerateMermaid converts the system architecture into a Mermaid flowchart definition.
func GenerateMermaid(arch *SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// 1. Define Nodes (Components)
	// Sort components by ID for deterministic output
	components := make([]Component, len(arch.Components))
	copy(components, arch.Components)
	sort.Slice(components, func(i, j int) bool {
		return components[i].ID < components[j].ID
	})

	for _, comp := range components {
		id := sanitizeMermaidID(comp.ID)
		label := sanitizeLabel(comp.ID) // Or use a nicer label if available? ID is usually the name.

		var shapeStart, shapeEnd string
		switch strings.ToLower(comp.Type) {
		case "database", "db", "storage":
			shapeStart, shapeEnd = "[(", ")]"
		case "queue", "topic", "bus":
			shapeStart, shapeEnd = "{{", "}}"
		case "worker", "job":
			shapeStart, shapeEnd = "([", "])"
		case "frontend", "ui", "web":
			shapeStart, shapeEnd = "((", "))"
		default:
			shapeStart, shapeEnd = "[", "]"
		}

		sb.WriteString(fmt.Sprintf("    %s%s\"%s\"%s\n", id, shapeStart, label, shapeEnd))
	}

	sb.WriteString("\n")

	// 2. Define Edges (Consumes/Produces)
	// Track edges to avoid duplicates if both sides declare it
	edges := make(map[string]bool)

	addEdge := func(from, to, label string) {
		key := fmt.Sprintf("%s->%s:%s", from, to, label)
		if edges[key] {
			return
		}
		edges[key] = true
		sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", from, label, to))
	}

	for _, comp := range components {
		myID := sanitizeMermaidID(comp.ID)

		// Consumes: Source -> Me
		for _, input := range comp.Consumes {
			if input.Source == "" {
				continue
			}
			srcID := sanitizeMermaidID(input.Source)
			label := input.Type
			if label == "" {
				label = "uses"
			}
			addEdge(srcID, myID, label)
		}

		// Produces: Me -> Target
		for _, output := range comp.Produces {
			if output.Target == "" {
				continue
			}
			targetID := sanitizeMermaidID(output.Target)
			label := output.Event
			if label == "" {
				label = output.Type
			}
			if label == "" {
				label = "sends"
			}
			addEdge(myID, targetID, label)
		}
	}

	return sb.String()
}

func sanitizeMermaidID(id string) string {
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, ".", "_")
	id = strings.ReplaceAll(id, "\"", "")
	return id
}

func sanitizeLabel(label string) string {
	return strings.ReplaceAll(label, "\"", "'")
}
