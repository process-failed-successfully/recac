package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/orchestrator"

	"github.com/charmbracelet/lipgloss"
)

func explainPipelineJob(filePath string, target string, vars map[string]string) {
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

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	jobStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	depStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	// Group jobs by dependencies
	// Create an adjacency list (who depends on whom)
	adj := make(map[string][]string)
	inDegree := make(map[string]int)

	// Map item.ID to WorkItem
	itemMap := make(map[string]orchestrator.WorkItem)

	for _, item := range items {
		itemMap[item.ID] = item
		inDegree[item.ID] = 0
	}

	for _, item := range items {
		for _, dep := range item.DependsOn {
			adj[dep] = append(adj[dep], item.ID)
			inDegree[item.ID]++
		}
	}

	// Kahn's algorithm for topological sort/layering
	var queue []string
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	var layers [][]string

	for len(queue) > 0 {
		var nextQueue []string
		var currentLayer []string

		for _, id := range queue {
			currentLayer = append(currentLayer, id)
			for _, dependent := range adj[id] {
				inDegree[dependent]--
				if inDegree[dependent] == 0 {
					nextQueue = append(nextQueue, dependent)
				}
			}
		}

		layers = append(layers, currentLayer)
		queue = nextQueue
	}

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Pipeline Explanation: %s", filePath)))
	if target != "" {
		fmt.Fprintf(stdout, "Target: %s (and its dependencies)\n", target)
	}
	fmt.Fprintln(stdout, "")

	for i, layer := range layers {
		fmt.Fprintf(stdout, "=== Layer %d ===\n", i+1)
		for _, id := range layer {
			item := itemMap[id]

			// Display
			fmt.Fprintf(stdout, "%s\n", jobStyle.Render(fmt.Sprintf("Job: %s", item.ID)))
			fmt.Fprintf(stdout, "  Summary: %s\n", item.Summary)
			if len(item.DependsOn) > 0 {
				fmt.Fprintf(stdout, "  Depends On: %s\n", depStyle.Render(strings.Join(item.DependsOn, ", ")))
			}
			if item.RunCondition != "" {
				fmt.Fprintf(stdout, "  Run Condition: %s\n", item.RunCondition)
			}

			// Display Matrix expanded vars if available in EnvVars
			if len(item.EnvVars) > 0 {
				var envs []string
				for k, v := range item.EnvVars {
					if !strings.HasPrefix(k, "RECAC_") {
						envs = append(envs, fmt.Sprintf("%s=%s", k, v))
					}
				}
				if len(envs) > 0 {
					fmt.Fprintf(stdout, "  Env: %s\n", strings.Join(envs, ", "))
				}
			}

			if len(item.Tags) > 0 {
				fmt.Fprintf(stdout, "  Tags: %s\n", strings.Join(item.Tags, ", "))
			}
			fmt.Fprintln(stdout, "")
		}
	}
}
