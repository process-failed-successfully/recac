package runner

import (
	"fmt"
	"recac/internal/utils"
	"sort"
	"strings"
)

// GenerateMermaid generates a Mermaid flowchart from the task graph.
func GenerateMermaid(g *TaskGraph) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Collect nodes to ensure deterministic output
	var nodes []*TaskNode
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
		case TaskDone:
			style = ":::done"
		case TaskInProgress:
			style = ":::inprogress"
		case TaskFailed:
			style = ":::failed"
		case TaskReady:
			style = ":::ready"
		default: // Pending
			style = ":::pending"
		}

		// Sanitize ID and Name for Mermaid
		safeID := utils.SanitizeMermaidID(node.ID)
		safeName := strings.ReplaceAll(node.Name, "\"", "'")
		safeName = strings.ReplaceAll(safeName, "\n", " ")
		if len(safeName) > 30 {
			safeName = safeName[:27] + "..."
		}

		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]%s\n", safeID, safeName, style))

		for _, depID := range node.Dependencies {
			safeDepID := utils.SanitizeMermaidID(depID)
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
