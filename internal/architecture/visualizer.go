package architecture

import (
	"fmt"
	"sort"
	"strings"
)

// GenerateMermaid generates a Mermaid diagram from the SystemArchitecture.
func GenerateMermaid(arch *SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// 1. Define Nodes
	// Copy components to avoid modifying the input
	comps := make([]Component, len(arch.Components))
	copy(comps, arch.Components)

	// Sort components by ID for deterministic output
	sort.Slice(comps, func(i, j int) bool {
		return comps[i].ID < comps[j].ID
	})

	for _, comp := range comps {
		id := sanitizeMermaidID(comp.ID)
		shape := getShape(id, comp.ID, comp.Type)
		sb.WriteString(fmt.Sprintf("    %s\n", shape))
	}

	sb.WriteString("\n")

	// 2. Define Edges
	// We iterate over the original or sorted list, doesn't matter for edges as we collect them first.
	// But using sorted list is slightly cleaner.

	type edge struct {
		src, tgt, label string
	}
	var edgeList []edge

	for _, comp := range comps {
		compID := sanitizeMermaidID(comp.ID)

		// Consumes: Source -> Component
		for _, input := range comp.Consumes {
			if input.Source != "" {
				srcID := sanitizeMermaidID(input.Source)
				label := input.Type
				if label == "" {
					label = "consumes"
				}
				edgeList = append(edgeList, edge{srcID, compID, label})
			}
		}

		// Produces: Component -> Target
		for _, output := range comp.Produces {
			if output.Target != "" {
				targetID := sanitizeMermaidID(output.Target)
				label := output.Event
				if label == "" {
					label = output.Type
				}
				if label == "" {
					label = "produces"
				}
				edgeList = append(edgeList, edge{compID, targetID, label})
			}
		}
	}

	// Merge labels for duplicate edges
	mergedEdges := make(map[string]string)
	for _, e := range edgeList {
		key := fmt.Sprintf("%s->%s", e.src, e.tgt)
		if existing, ok := mergedEdges[key]; ok {
			// Check if label already exists in the comma-separated string
			// Simple check: splitting by ", "
			parts := strings.Split(existing, ", ")
			exists := false
			for _, p := range parts {
				if p == e.label {
					exists = true
					break
				}
			}
			if !exists {
				mergedEdges[key] = existing + ", " + e.label
			}
		} else {
			mergedEdges[key] = e.label
		}
	}

	// Sort edges by key (src->tgt)
	var sortedKeys []string
	for k := range mergedEdges {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, key := range sortedKeys {
		parts := strings.Split(key, "->")
		if len(parts) != 2 {
			continue
		}
		src, tgt := parts[0], parts[1]
		label := mergedEdges[key]
		sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", src, label, tgt))
	}

	return sb.String()
}

func sanitizeMermaidID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, ".", "_")
	return id
}

func getShape(id, label, compType string) string {
	t := strings.ToLower(strings.TrimSpace(compType))

	// Escape quotes in label if necessary, though simpler to assume no quotes in ID
	label = strings.ReplaceAll(label, "\"", "'")

	switch t {
	case "database", "db", "storage":
		// Cylinder: [()]
		return fmt.Sprintf("%s[(\"%s\")]", id, label)
	case "queue", "topic", "stream":
		// Hexagon: {{}}
		return fmt.Sprintf("%s{{\"%s\"}}", id, label)
	case "worker", "consumer":
		// Rounded: ([])
		return fmt.Sprintf("%s([\"%s\"])", id, label)
	case "frontend", "ui", "web":
		// Circle: (())
		return fmt.Sprintf("%s((\"%s\"))", id, label)
	default:
		// Default Rect: []
		return fmt.Sprintf("%s[\"%s\"]", id, label)
	}
}
