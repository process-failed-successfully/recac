package architecture

import (
	"fmt"
	"strings"
)

// GenerateMermaid converts a SystemArchitecture into a Mermaid graph string.
func GenerateMermaid(arch *SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Helper to sanitize IDs for Mermaid compatibility
	sanitizeID := func(id string) string {
		return strings.NewReplacer(
			" ", "_",
			"-", "_",
			".", "_",
		).Replace(id)
	}

	// 1. Define Nodes
	for _, c := range arch.Components {
		shapeOpen, shapeClose := "[", "]"
		switch strings.ToLower(c.Type) {
		case "database":
			shapeOpen, shapeClose = "[(", ")]"
		case "queue":
			shapeOpen, shapeClose = "{{", "}}"
		case "worker":
			shapeOpen, shapeClose = "([", "])"
		case "frontend":
			shapeOpen, shapeClose = "((", "))"
		}

		safeID := sanitizeID(c.ID)
		// Use ID as label if Description is too long, or just ID
		label := c.ID
		sb.WriteString(fmt.Sprintf("    %s%s\"%s\"%s\n", safeID, shapeOpen, label, shapeClose))
	}

	// 2. Define Edges
	// We iterate components to find relationships

	// Map to track edges to avoid duplicates if necessary, though Mermaid handles duplicates fine (draws multiple lines)
	// For clarity, we'll just iterate consumers and producers.

	for _, c := range arch.Components {
		compID := sanitizeID(c.ID)

		// Consumes: Source -> Component
		for _, inp := range c.Consumes {
			if inp.Source == "" {
				continue
			}
			sourceID := sanitizeID(inp.Source)

			// Label logic
			label := inp.Type
			if label == "" {
				label = "uses" // default label
			}

			// Add edge
			sb.WriteString(fmt.Sprintf("    %s -- \"%s\" --> %s\n", sourceID, label, compID))
		}

		// Produces: Component -> Target (if explicit)
		for _, out := range c.Produces {
			if out.Target == "" {
				continue
			}
			targetID := sanitizeID(out.Target)

			// Label logic
			label := out.Event
			if label == "" {
				label = out.Type
			}
			if label == "" {
				label = "produces"
			}

			// Add edge
			sb.WriteString(fmt.Sprintf("    %s -- \"%s\" --> %s\n", compID, label, targetID))
		}
	}

	// 3. Handle External Nodes (optional, if source/target not in Components)
	// We can iterate again or use a set to track known components.
	// For now, Mermaid will just create a default node if an ID is used in an edge but not defined.
	// To make them look different (e.g. dashed), we could check validity.

	knownIDs := make(map[string]bool)
	for _, c := range arch.Components {
		knownIDs[sanitizeID(c.ID)] = true
	}

	// Scan edges again to style external nodes?
	// This requires building an edge list first.
	// Let's keep it simple for v1: implicit nodes are rendered as default boxes.

	// However, if we want to style external dependencies (e.g. "auth0"), we could identify them.
	for _, c := range arch.Components {
		for _, inp := range c.Consumes {
			src := sanitizeID(inp.Source)
			if !knownIDs[src] && src != "" {
				sb.WriteString(fmt.Sprintf("    style %s stroke-dasharray 5 5\n", src))
				knownIDs[src] = true // mark as styled
			}
		}
		for _, out := range c.Produces {
			tgt := sanitizeID(out.Target)
			if !knownIDs[tgt] && tgt != "" {
				sb.WriteString(fmt.Sprintf("    style %s stroke-dasharray 5 5\n", tgt))
				knownIDs[tgt] = true
			}
		}
	}

	return sb.String()
}
