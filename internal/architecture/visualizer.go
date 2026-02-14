package architecture

import (
	"fmt"
	"strings"
)

// GenerateMermaid creates a Mermaid flowchart from the system architecture.
func GenerateMermaid(arch *SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("flowchart TD\n")

	// Map to track edges and prevent duplicates: key = "Source|Target|Label"
	edges := make(map[string]bool)

	// 1. Define Nodes (Components)
	for _, comp := range arch.Components {
		safeID := sanitizeID(comp.ID)
		label := fmt.Sprintf("<b>%s</b><br/><i>%s</i>", comp.ID, comp.Type)
		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", safeID, label))
	}

	sb.WriteString("\n")

	// 2. Define Edges (Relationships)
	for _, comp := range arch.Components {
		safeID := sanitizeID(comp.ID)

		// Consumes: Source -> This Component
		for _, input := range comp.Consumes {
			if input.Source != "" {
				safeSource := sanitizeID(input.Source)
				label := input.Type
				if label == "" {
					label = "uses"
				}

				edgeKey := fmt.Sprintf("%s|%s|%s", safeSource, safeID, label)
				if !edges[edgeKey] {
					sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", safeSource, label, safeID))
					edges[edgeKey] = true
				}
			}
		}

		// Produces: This Component -> Target (if explicit)
		for _, output := range comp.Produces {
			if output.Target != "" {
				safeTarget := sanitizeID(output.Target)
				label := output.Event
				if label == "" {
					label = output.Type
				}
				if label == "" {
					label = "calls"
				}

				edgeKey := fmt.Sprintf("%s|%s|%s", safeID, safeTarget, label)
				if !edges[edgeKey] {
					sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", safeID, label, safeTarget))
					edges[edgeKey] = true
				}
			}
		}
	}

	return sb.String()
}

func sanitizeID(id string) string {
	return strings.ReplaceAll(id, "-", "_")
}
