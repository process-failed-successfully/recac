package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/orchestrator"
)

func exportPipelineGraphJob(filePath string, target string, vars map[string]string, format string, outFile string) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to read file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}

	importDir := "."
	if filePath != "" {
		importDir = filepath.Dir(filePath)
	}

	items, err := orchestrator.ParsePipelineToWorkItems(fileData, target, vars, importDir)
	if err != nil {
		fmt.Fprintf(stdout, "Pipeline validation failed: %v\n", err)
		exitFunc(1)
		return
	}

	var jobs []orchestrator.JobInfo
	for _, item := range items {
		jobs = append(jobs, orchestrator.JobInfo{
			ID:       item.ID,
			WorkItem: item,
			Status:   "Pending",
		})
	}

	var graphOutput string
	// ⚡ Bolt: Replace switch strings.ToLower with allocation-free strings.EqualFold
	if strings.EqualFold(format, "mermaid") {
		graphOutput = orchestrator.ExportGraphToMermaid(jobs)
	} else if strings.EqualFold(format, "plantuml") {
		graphOutput = orchestrator.ExportGraphToPlantUML(jobs)
	} else if strings.EqualFold(format, "dot") {
		graphOutput = orchestrator.ExportGraphToDOT(jobs)
	} else {
		fmt.Fprintf(stdout, "Unsupported graph format: %s. Use 'mermaid', 'plantuml', or 'dot'.\n", format)
		exitFunc(1)
		return
	}

	if outFile == "" || outFile == "-" {
		fmt.Fprintln(stdout, graphOutput)
	} else {
		err := os.WriteFile(outFile, []byte(graphOutput), 0644)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to write to file %s: %v\n", outFile, err)
			exitFunc(1)
			return
		}
		fmt.Fprintf(stdout, "Graph exported successfully to %s\n", outFile)
	}
}
