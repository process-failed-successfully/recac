package architecture

import (
	"fmt"
	"strings"
)

// GenerateMermaid generates a Mermaid diagram string from the SystemArchitecture.
func GenerateMermaid(arch *SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Helper to sanitize IDs for Mermaid
	sanitizeID := func(id string) string {
		id = strings.ReplaceAll(id, " ", "_")
		id = strings.ReplaceAll(id, "-", "_")
		id = strings.ReplaceAll(id, ".", "_")
		id = strings.ReplaceAll(id, "\"", "_")
		return id
	}

	// Helper to sanitize labels (escape quotes)
	sanitizeLabel := func(label string) string {
		return strings.ReplaceAll(label, "\"", "\\\"")
	}

	// 1. Define Nodes
	for _, comp := range arch.Components {
		id := sanitizeID(comp.ID)
		label := sanitizeLabel(comp.ID)
		shape := id // Default to square

		switch strings.ToLower(comp.Type) {
		case "database", "db":
			shape = fmt.Sprintf("%s[(\"%s\")]", id, label)
		case "queue", "topic", "stream":
			shape = fmt.Sprintf("%s{{\"%s\"}}", id, label)
		case "worker", "job":
			shape = fmt.Sprintf("%s([\"%s\"])", id, label)
		case "frontend", "ui":
			shape = fmt.Sprintf("%s((\"%s\"))", id, label)
		default:
			shape = fmt.Sprintf("%s[\"%s\"]", id, label)
		}
		sb.WriteString(fmt.Sprintf("    %s\n", shape))
	}

	sb.WriteString("\n")

	// 2. Define Edges (Consumes)
	for _, comp := range arch.Components {
		targetID := sanitizeID(comp.ID)
		for _, input := range comp.Consumes {
			sourceID := sanitizeID(input.Source)
			label := input.Type
			if label == "" {
				label = input.Schema
			}
			if label == "" {
				label = "uses"
			}
			label = sanitizeLabel(label)
			sb.WriteString(fmt.Sprintf("    %s -->|\"%s\"| %s\n", sourceID, label, targetID))
		}
	}

	// 3. Define Edges (Produces - if explicit target)
	for _, comp := range arch.Components {
		sourceID := sanitizeID(comp.ID)
		for _, output := range comp.Produces {
			if output.Target != "" {
				targetID := sanitizeID(output.Target)
				label := output.Event
				if label == "" {
					label = output.Type
				}
				if label == "" {
					label = "triggers"
				}
				label = sanitizeLabel(label)
				sb.WriteString(fmt.Sprintf("    %s -->|\"%s\"| %s\n", sourceID, label, targetID))
			}
		}
	}

	return sb.String()
}
