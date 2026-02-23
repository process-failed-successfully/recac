package architecture

import (
	"fmt"
	"sort"
	"strings"
)

// GenerateMermaid converts a SystemArchitecture into a Mermaid graph string.
func GenerateMermaid(arch *SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	sanitizeID := func(id string) string {
		id = strings.ReplaceAll(id, " ", "_")
		id = strings.ReplaceAll(id, "-", "_")
		id = strings.ReplaceAll(id, ".", "_")
		id = strings.ReplaceAll(id, "\"", "_")
		return id
	}

	sanitizeLabel := func(label string) string {
		return strings.ReplaceAll(label, "\"", "#quot;")
	}

	// 1. Define Nodes
	// Create a copy to sort without modifying original
	sortedComps := make([]Component, len(arch.Components))
	copy(sortedComps, arch.Components)
	sort.Slice(sortedComps, func(i, j int) bool {
		return sortedComps[i].ID < sortedComps[j].ID
	})

	for _, comp := range sortedComps {
		id := sanitizeID(comp.ID)
		label := sanitizeLabel(comp.ID)

		switch strings.ToLower(comp.Type) {
		case "database":
			sb.WriteString(fmt.Sprintf("    %s[(\"%s\")]\n", id, label))
		case "queue":
			sb.WriteString(fmt.Sprintf("    %s{{\"%s\"}}\n", id, label))
		case "worker":
			sb.WriteString(fmt.Sprintf("    %s([\"%s\"])\n", id, label))
		case "frontend":
			sb.WriteString(fmt.Sprintf("    %s((\"%s\"))\n", id, label))
		default: // service or others
			sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", id, label))
		}
	}

	// 2. Collect Edges
	type edge struct {
		from, to, label string
	}
	edgeMap := make(map[string]edge)

	addEdge := func(from, to, label string) {
		fromID := sanitizeID(from)
		toID := sanitizeID(to)
		key := fmt.Sprintf("%s|%s", fromID, toID)

		if existing, ok := edgeMap[key]; ok {
			// Merge logic
			if existing.label != "" && label != "" && existing.label != label {
				// Simple check to avoid exact duplicates
				if !strings.Contains(existing.label, label) {
					newLabel := existing.label + ", " + label
					edgeMap[key] = edge{fromID, toID, newLabel}
				}
			} else if existing.label == "" && label != "" {
				edgeMap[key] = edge{fromID, toID, label}
			}
		} else {
			edgeMap[key] = edge{fromID, toID, label}
		}
	}

	// From Consumes
	for _, consumer := range arch.Components {
		for _, input := range consumer.Consumes {
			if input.Source != "" {
				addEdge(input.Source, consumer.ID, input.Type)
			}
		}
	}

	// From Produces
	for _, producer := range arch.Components {
		for _, output := range producer.Produces {
			if output.Target != "" {
				label := output.Event
				if label == "" {
					label = output.Type
				}
				addEdge(producer.ID, output.Target, label)
			}
		}
	}

	// 3. Write Edges (Sorted)
	var edges []edge
	for _, e := range edgeMap {
		edges = append(edges, e)
	}

	sort.Slice(edges, func(i, j int) bool {
		if edges[i].from != edges[j].from {
			return edges[i].from < edges[j].from
		}
		return edges[i].to < edges[j].to
	})

	for _, e := range edges {
		if e.label != "" {
			sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", e.from, e.label, e.to))
		} else {
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", e.from, e.to))
		}
	}

	return sb.String()
}
