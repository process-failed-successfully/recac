package architecture

import (
	"fmt"
	"sort"
	"strings"
)

// GenerateMermaid generates a Mermaid graph from the SystemArchitecture.
func GenerateMermaid(arch *SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	systemTitle := arch.SystemName
	if systemTitle == "" {
		systemTitle = "System"
	}
	sb.WriteString(fmt.Sprintf("    subgraph System [\"%s\"]\n", systemTitle))

	// Collect nodes
	var componentIDs []string
	for _, c := range arch.Components {
		componentIDs = append(componentIDs, c.ID)
	}
	sort.Strings(componentIDs) // Deterministic order

	// Map component ID to full struct for easy access
	compMap := make(map[string]Component)
	for _, c := range arch.Components {
		compMap[c.ID] = c
	}

	// Render nodes
	for _, id := range componentIDs {
		c := compMap[id]
		shapeOpen := "["
		shapeClose := "]"
		if c.Type == "database" {
			shapeOpen = "[("
			shapeClose = ")]"
		} else if c.Type == "worker" {
			shapeOpen = "(("
			shapeClose = "))"
		}

		// Escape description for tooltip or just use name+type
		label := fmt.Sprintf("%s<br/>(%s)", c.ID, c.Type)
		sb.WriteString(fmt.Sprintf("    %s%s\"%s\"%s\n", c.ID, shapeOpen, label, shapeClose))
	}

	sb.WriteString("    end\n\n")

	// Collect edges
	type Edge struct {
		From, To, Label string
	}
	edgeMap := make(map[string]Edge)

	for _, id := range componentIDs {
		c := compMap[id]

		// Consumes: Source -> Self
		for _, input := range c.Consumes {
			if _, exists := compMap[input.Source]; exists {
				key := fmt.Sprintf("%s->%s:%s", input.Source, c.ID, input.Type)
				edgeMap[key] = Edge{
					From:  input.Source,
					To:    c.ID,
					Label: input.Type,
				}
			}
		}

		// Produces: Self -> Target
		for _, output := range c.Produces {
			if output.Target != "" {
				if _, exists := compMap[output.Target]; exists {
					key := fmt.Sprintf("%s->%s:%s", c.ID, output.Target, output.Type)
					edgeMap[key] = Edge{
						From:  c.ID,
						To:    output.Target,
						Label: output.Type,
					}
				}
			}
		}
	}

	// Convert map to slice for sorting
	var edges []Edge
	for _, e := range edgeMap {
		edges = append(edges, e)
	}

	// Sort edges for determinism
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Label < edges[j].Label
	})

	// Render edges
	for _, e := range edges {
		label := ""
		if e.Label != "" {
			label = fmt.Sprintf("|%s|", e.Label)
		}
		sb.WriteString(fmt.Sprintf("    %s -->%s %s\n", e.From, label, e.To))
	}

	return sb.String()
}
