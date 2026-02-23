package visualize

import (
	"fmt"
	"recac/internal/runner"
	"sort"
	"strings"
)

// GenerateMermaid generates a Mermaid flowchart from a TaskGraph.
func GenerateMermaid(g *runner.TaskGraph) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Collect nodes to ensure deterministic output
	var nodes []*runner.TaskNode
	for _, node := range g.Nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	for _, node := range nodes {
		// Style based on status
		style := ""
		switch node.Status {
		case runner.TaskDone:
			style = ":::done"
		case runner.TaskInProgress:
			style = ":::inprogress"
		case runner.TaskFailed:
			style = ":::failed"
		case runner.TaskReady:
			style = ":::ready"
		default: // Pending
			style = ":::pending"
		}

		// Sanitize ID and Name for Mermaid
		safeID := SanitizeID(node.ID)
		safeName := strings.ReplaceAll(node.Name, "\"", "'")
		safeName = strings.ReplaceAll(safeName, "\n", " ")
		if len(safeName) > 30 {
			safeName = safeName[:27] + "..."
		}

		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]%s\n", safeID, safeName, style))

		for _, depID := range node.Dependencies {
			safeDepID := SanitizeID(depID)
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeDepID, safeID))
		}
	}

	// Legend/Styles
	sb.WriteString("\n    classDef done fill:#90EE90,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef inprogress fill:#87CEEB,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef failed fill:#FF6347,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef ready fill:#FFD700,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef pending fill:#D3D3D3,stroke:#333,stroke-width:1px,color:black;\n")

	return sb.String()
}

// SanitizeID sanitizes a string for use as a Mermaid node ID.
func SanitizeID(id string) string {
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, ".", "_")
	return id
}
