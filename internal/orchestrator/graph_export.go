package orchestrator

import (
	"fmt"
	"strings"
)

// sanitizeMermaidNodeName is a helper to sanitize job IDs for Mermaid node names
func sanitizeMermaidNodeName(id string) string {
	var sb strings.Builder
	sb.Grow(len(id))
	for i := 0; i < len(id); i++ {
		c := id[i]
		if c == '-' || c == '.' {
			sb.WriteByte('_')
		} else {
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

// ExportGraphToMermaid converts a list of jobs into a Mermaid.js flowchart representation.
func ExportGraphToMermaid(jobs []JobInfo) string {
	var builder strings.Builder

	builder.WriteString("graph TD;\n")

	// Output nodes with styling classes based on status
	for _, job := range jobs {
		nodeName := sanitizeMermaidNodeName(job.ID)

		// Map status to Mermaid classes
		// ⚡ Bolt: Replaced switch strings.ToLower with allocation-free strings.EqualFold
		statusClass := "default"
		if strings.EqualFold(job.Status, "completed") {
			statusClass = "completed"
		} else if strings.EqualFold(job.Status, "failed") || strings.EqualFold(job.Status, "error") {
			statusClass = "failed"
		} else if strings.EqualFold(job.Status, "running") || strings.EqualFold(job.Status, "active") || strings.EqualFold(job.Status, "spawning") {
			statusClass = "running"
		} else if strings.EqualFold(job.Status, "pending") || strings.EqualFold(job.Status, "pending approval") {
			statusClass = "pending"
		} else if strings.EqualFold(job.Status, "canceled") {
			statusClass = "canceled"
		}

		builder.WriteString(fmt.Sprintf("    %s[\"%s\n(%s)\"]:::%s;\n", nodeName, job.ID, job.Status, statusClass))
	}

	// Output edges
	for _, job := range jobs {
		nodeName := sanitizeMermaidNodeName(job.ID)
		for _, dep := range job.WorkItem.DependsOn {
			// Find dependency in the job list to ensure it exists
			depExists := false
			for _, dJob := range jobs {
				if dJob.ID == dep {
					depExists = true
					break
				}
			}
			if depExists {
				depNodeName := sanitizeMermaidNodeName(dep)
				builder.WriteString(fmt.Sprintf("    %s --> %s;\n", depNodeName, nodeName))
			}
		}
	}

	// Define styles
	builder.WriteString("\n    classDef default fill:#f9f9f9,stroke:#333,stroke-width:2px;\n")
	builder.WriteString("    classDef completed fill:#d4edda,stroke:#4caf50,stroke-width:2px;\n")
	builder.WriteString("    classDef failed fill:#f8d7da,stroke:#f44336,stroke-width:2px;\n")
	builder.WriteString("    classDef running fill:#cce5ff,stroke:#2196f3,stroke-width:2px;\n")
	builder.WriteString("    classDef pending fill:#fff3cd,stroke:#ff9800,stroke-width:2px;\n")
	builder.WriteString("    classDef canceled fill:#e2e3e5,stroke:#9e9e9e,stroke-width:2px;\n")

	return builder.String()
}

// sanitizePlantUMLNodeName is a helper to sanitize job IDs for PlantUML node names
func sanitizePlantUMLNodeName(id string) string {
	var sb strings.Builder
	sb.Grow(len(id))
	for i := 0; i < len(id); i++ {
		c := id[i]
		if c == '-' || c == '.' {
			sb.WriteByte('_')
		} else {
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

// ExportGraphToPlantUML converts a list of jobs into a PlantUML representation.
func ExportGraphToPlantUML(jobs []JobInfo) string {
	var builder strings.Builder

	builder.WriteString("@startuml\n")
	builder.WriteString("skinparam componentStyle rectangle\n\n")

	// Output nodes with styling based on status
	for _, job := range jobs {
		nodeName := sanitizePlantUMLNodeName(job.ID)

		color := "#LightGray"
		// ⚡ Bolt: Replaced switch strings.ToLower with allocation-free strings.EqualFold
		if strings.EqualFold(job.Status, "completed") {
			color = "#LightGreen"
		} else if strings.EqualFold(job.Status, "failed") || strings.EqualFold(job.Status, "error") {
			color = "#LightCoral"
		} else if strings.EqualFold(job.Status, "running") || strings.EqualFold(job.Status, "active") || strings.EqualFold(job.Status, "spawning") {
			color = "#LightBlue"
		} else if strings.EqualFold(job.Status, "pending") || strings.EqualFold(job.Status, "pending approval") {
			color = "#LightYellow"
		} else if strings.EqualFold(job.Status, "canceled") {
			color = "#Gainsboro"
		}

		label := fmt.Sprintf("%s\\n(%s)", job.ID, job.Status)
		builder.WriteString(fmt.Sprintf("component \"%s\" as %s %s\n", label, nodeName, color))
	}

	builder.WriteString("\n")

	// Output edges
	for _, job := range jobs {
		nodeName := sanitizePlantUMLNodeName(job.ID)
		for _, dep := range job.WorkItem.DependsOn {
			// Find dependency in the job list
			depExists := false
			for _, dJob := range jobs {
				if dJob.ID == dep {
					depExists = true
					break
				}
			}
			if depExists {
				depNodeName := sanitizePlantUMLNodeName(dep)
				builder.WriteString(fmt.Sprintf("%s --> %s\n", depNodeName, nodeName))
			}
		}
	}

	builder.WriteString("@enduml\n")
	return builder.String()
}

// sanitizeDotNodeName is a helper to sanitize job IDs for DOT node names
func sanitizeDotNodeName(id string) string {
	var sb strings.Builder
	sb.Grow(len(id) + 2)
	sb.WriteByte('"')
	for i := 0; i < len(id); i++ {
		c := id[i]
		if c == '-' || c == '.' {
			sb.WriteByte('_')
		} else {
			sb.WriteByte(c)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

// ExportGraphToDOT converts a list of jobs into a Graphviz DOT representation.
func ExportGraphToDOT(jobs []JobInfo) string {
	var builder strings.Builder

	builder.WriteString("digraph G {\n")
	builder.WriteString("    node [shape=box, style=filled];\n")

	// Output nodes with styling based on status
	for _, job := range jobs {
		nodeName := sanitizeDotNodeName(job.ID)

		color := "lightgray"
		// ⚡ Bolt: Replaced switch strings.ToLower with allocation-free strings.EqualFold
		if strings.EqualFold(job.Status, "completed") {
			color = "lightgreen"
		} else if strings.EqualFold(job.Status, "failed") || strings.EqualFold(job.Status, "error") {
			color = "lightcoral"
		} else if strings.EqualFold(job.Status, "running") || strings.EqualFold(job.Status, "active") || strings.EqualFold(job.Status, "spawning") {
			color = "lightblue"
		} else if strings.EqualFold(job.Status, "pending") || strings.EqualFold(job.Status, "pending approval") {
			color = "lightyellow"
		} else if strings.EqualFold(job.Status, "canceled") {
			color = "gainsboro"
		}

		label := fmt.Sprintf("%s\\n(%s)", job.ID, job.Status)
		builder.WriteString(fmt.Sprintf("    %s [label=\"%s\", fillcolor=\"%s\"];\n", nodeName, label, color))
	}

	// Output edges
	for _, job := range jobs {
		nodeName := sanitizeDotNodeName(job.ID)
		for _, dep := range job.WorkItem.DependsOn {
			// Find dependency in the job list
			depExists := false
			for _, dJob := range jobs {
				if dJob.ID == dep {
					depExists = true
					break
				}
			}
			if depExists {
				depNodeName := sanitizeDotNodeName(dep)
				builder.WriteString(fmt.Sprintf("    %s -> %s;\n", depNodeName, nodeName))
			}
		}
	}

	builder.WriteString("}\n")
	return builder.String()
}
