package architecture

import (
	"fmt"
	"sort"
	"strings"
)

// GenerateMermaid generates a Mermaid flowchart from the system architecture.
func GenerateMermaid(arch *SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Sort components by ID to ensure deterministic output
	components := make([]Component, len(arch.Components))
	copy(components, arch.Components)
	sort.Slice(components, func(i, j int) bool {
		return components[i].ID < components[j].ID
	})

	// Generate Nodes
	for _, comp := range components {
		id := sanitizeMermaidID(comp.ID)
		label := sanitizeLabel(comp.ID)
		if comp.Description != "" {
			// Add type to label for clarity
			label = fmt.Sprintf("%s\\n(%s)", comp.ID, comp.Type)
		} else {
			label = fmt.Sprintf("%s", comp.ID)
		}

		// Use different shapes/styles based on type
		shapeOpen := "("
		shapeClose := ")"
		style := ":::service"

		switch strings.ToLower(comp.Type) {
		case "database", "db", "storage", "postgres", "mysql", "redis", "mongo":
			shapeOpen = "[("
			shapeClose = ")]"
			style = ":::database"
		case "queue", "topic", "stream", "kafka", "rabbitmq":
			shapeOpen = "{{"
			shapeClose = "}}"
			style = ":::queue"
		case "frontend", "ui", "web", "spa", "mobile":
			shapeOpen = "[/"
			shapeClose = "/]"
			style = ":::frontend"
		case "worker", "job", "cron", "daemon":
			shapeOpen = "[/"
			shapeClose = "\\]"
			style = ":::worker"
		default: // service, api, backend
			shapeOpen = "("
			shapeClose = ")"
			style = ":::service"
		}

		sb.WriteString(fmt.Sprintf("    %s%s\"%s\"%s%s\n", id, shapeOpen, label, shapeClose, style))
	}

	// Generate Edges
	// We use a map to deduplicate edges: "from->to" -> label
	edges := make(map[string]string)

	for _, comp := range components {
		consumerID := sanitizeMermaidID(comp.ID)

		// Consumes: Source -> Component
		for _, input := range comp.Consumes {
			if input.Source != "" {
				producerID := sanitizeMermaidID(input.Source)
				edgeKey := fmt.Sprintf("%s->%s", producerID, consumerID)

				// Append label if exists (handle multiple types/events on same edge)
				label := ""
				if input.Type != "" {
					label = input.Type
				}

				if existing, ok := edges[edgeKey]; ok && existing != "" {
					if label != "" {
						// Check if label already exists
						parts := strings.Split(existing, ", ")
						found := false
						for _, p := range parts {
							if p == label {
								found = true
								break
							}
						}
						if !found {
							edges[edgeKey] = existing + ", " + label
						}
					}
				} else {
					edges[edgeKey] = label
				}
			}
		}

		// Produces: Component -> Target
		for _, output := range comp.Produces {
			if output.Target != "" {
				producerID := sanitizeMermaidID(comp.ID)
				targetID := sanitizeMermaidID(output.Target)
				edgeKey := fmt.Sprintf("%s->%s", producerID, targetID)

				label := ""
				if output.Event != "" {
					label = output.Event
				} else if output.Type != "" {
					label = output.Type
				}

				if existing, ok := edges[edgeKey]; ok && existing != "" {
					if label != "" {
						// Check if label already exists
						parts := strings.Split(existing, ", ")
						found := false
						for _, p := range parts {
							if p == label {
								found = true
								break
							}
						}
						if !found {
							edges[edgeKey] = existing + ", " + label
						}
					}
				} else {
					edges[edgeKey] = label
				}
			}
		}
	}

	// Sort edges for deterministic output
	var edgeKeys []string
	for k := range edges {
		edgeKeys = append(edgeKeys, k)
	}
	sort.Strings(edgeKeys)

	for _, k := range edgeKeys {
		parts := strings.Split(k, "->")
		from := parts[0]
		to := parts[1]
		label := edges[k]

		arrow := "-->"
		if label != "" {
			arrow = fmt.Sprintf("-->|%s|", label)
		}
		sb.WriteString(fmt.Sprintf("    %s %s %s\n", from, arrow, to))
	}

	// Styles
	sb.WriteString("\n")
	sb.WriteString("    classDef service fill:#f9f,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef database fill:#ff9,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef queue fill:#9ff,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef frontend fill:#9f9,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef worker fill:#f99,stroke:#333,stroke-width:2px,color:black;\n")

	return sb.String()
}

func sanitizeMermaidID(id string) string {
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, ".", "_")
	id = strings.ReplaceAll(id, "\"", "_")
	return id
}

func sanitizeLabel(label string) string {
	return strings.ReplaceAll(label, "\"", "'")
}
