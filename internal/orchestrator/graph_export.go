package orchestrator

import (
	"fmt"
	"strings"
)

// ExportGraphToMermaid converts a list of jobs into a Mermaid.js flowchart representation.
func ExportGraphToMermaid(jobs []JobInfo) string {
	var builder strings.Builder

	builder.WriteString("graph TD;\n")

	// Helper to sanitize job IDs for Mermaid node names
	sanitizeNodeName := func(id string) string {
		s := strings.ReplaceAll(id, "-", "_")
		s = strings.ReplaceAll(s, ".", "_")
		return s
	}

	// Output nodes with styling classes based on status
	for _, job := range jobs {
		nodeName := sanitizeNodeName(job.ID)

		// Map status to Mermaid classes
		statusClass := "default"
		switch strings.ToLower(job.Status) {
		case "completed":
			statusClass = "completed"
		case "failed", "error":
			statusClass = "failed"
		case "running", "active", "spawning":
			statusClass = "running"
		case "pending", "pending approval":
			statusClass = "pending"
		case "canceled":
			statusClass = "canceled"
		}

		builder.WriteString(fmt.Sprintf("    %s[\"%s\n(%s)\"]:::%s;\n", nodeName, job.ID, job.Status, statusClass))
	}

	// Output edges
	for _, job := range jobs {
		nodeName := sanitizeNodeName(job.ID)
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
				depNodeName := sanitizeNodeName(dep)
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

// ExportGraphToDOT converts a list of jobs into a Graphviz DOT representation.
func ExportGraphToDOT(jobs []JobInfo) string {
	var builder strings.Builder

	builder.WriteString("digraph G {\n")
	builder.WriteString("    node [shape=box, style=filled];\n")

	// Helper to sanitize job IDs for DOT node names
	sanitizeNodeName := func(id string) string {
		s := strings.ReplaceAll(id, "-", "_")
		s = strings.ReplaceAll(s, ".", "_")
		return "\"" + s + "\""
	}

	// Output nodes with styling based on status
	for _, job := range jobs {
		nodeName := sanitizeNodeName(job.ID)

		color := "lightgray"
		switch strings.ToLower(job.Status) {
		case "completed":
			color = "lightgreen"
		case "failed", "error":
			color = "lightcoral"
		case "running", "active", "spawning":
			color = "lightblue"
		case "pending", "pending approval":
			color = "lightyellow"
		case "canceled":
			color = "gainsboro"
		}

		label := fmt.Sprintf("%s\\n(%s)", job.ID, job.Status)
		builder.WriteString(fmt.Sprintf("    %s [label=\"%s\", fillcolor=\"%s\"];\n", nodeName, label, color))
	}

	// Output edges
	for _, job := range jobs {
		nodeName := sanitizeNodeName(job.ID)
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
				depNodeName := sanitizeNodeName(dep)
				builder.WriteString(fmt.Sprintf("    %s -> %s;\n", depNodeName, nodeName))
			}
		}
	}

	builder.WriteString("}\n")
	return builder.String()
}
