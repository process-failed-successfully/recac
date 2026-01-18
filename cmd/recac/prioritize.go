package main

import (
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/db"
	"recac/internal/runner"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	prioritizeDryRun bool
	prioritizeOutput string
)

func init() {
	rootCmd.AddCommand(prioritizeCmd)
	prioritizeCmd.Flags().BoolVar(&prioritizeDryRun, "dry-run", false, "Simulate prioritization without saving changes")
	prioritizeCmd.Flags().StringVarP(&prioritizeOutput, "output", "o", "", "Output file (default: overwrite input file)")
}

var prioritizeCmd = &cobra.Command{
	Use:   "prioritize [plan_file]",
	Short: "Optimize and reorder the feature plan based on dependencies and priorities",
	Long: `Analyzes the feature plan (default: feature_list.json) and reorders tasks.
It ensures that:
1. All dependencies are satisfied before a task is listed.
2. High priority tasks ("Production", "Critical", "High") come before lower priority ones ("MVP", "POC", "Low") when dependencies allow.
3. Cycles and missing dependencies are detected.

Use --dry-run to preview the order without modifying the file.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		planFile := "feature_list.json"
		if len(args) > 0 {
			planFile = args[0]
		}

		// Read Plan
		content, err := os.ReadFile(planFile)
		if err != nil {
			return fmt.Errorf("failed to read plan file %s: %w", planFile, err)
		}

		var featureList db.FeatureList
		if err := json.Unmarshal(content, &featureList); err != nil {
			return fmt.Errorf("failed to parse plan JSON: %w", err)
		}

		// Create TaskGraph
		g := runner.NewTaskGraph()
		if err := g.LoadFromFeatures(featureList.Features); err != nil {
			return fmt.Errorf("failed to load features into graph: %w", err)
		}

		// Check for cycles
		if cycle, err := g.DetectCycles(); err != nil {
			return fmt.Errorf("cannot prioritize: %w", err)
		} else if len(cycle) > 0 {
			return fmt.Errorf("circular dependency detected: %v", cycle)
		}

		// Custom Topological Sort with Priority
		sortedIDs, err := topologicalSortWithPriority(g)
		if err != nil {
			return fmt.Errorf("failed to sort tasks: %w", err)
		}

		// Reorder Features
		featureMap := make(map[string]db.Feature)
		for _, f := range featureList.Features {
			featureMap[f.ID] = f
		}

		var newFeatures []db.Feature
		for _, id := range sortedIDs {
			if f, ok := featureMap[id]; ok {
				newFeatures = append(newFeatures, f)
			}
		}

		// Check for orphaned features (features that were in the list but not in the graph or sort result)
		// This shouldn't happen if LoadFromFeatures and sort logic covers everything, but good to be safe.
		if len(newFeatures) != len(featureList.Features) {
			cmd.PrintErrln("Warning: Some features were lost during sorting (orphaned?). Check integrity.")
		}

		featureList.Features = newFeatures

		// Output
		if prioritizeDryRun {
			cmd.Println("Dry run enabled. Proposed order:")
			for i, f := range featureList.Features {
				cmd.Printf("%d. [%s] %s (%s)\n", i+1, f.Priority, f.ID, f.Description)
			}
			return nil
		}

		outputFile := planFile
		if prioritizeOutput != "" {
			outputFile = prioritizeOutput
		}

		data, err := json.MarshalIndent(featureList, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal optimized plan: %w", err)
		}

		if err := os.WriteFile(outputFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write optimized plan to %s: %w", outputFile, err)
		}

		cmd.Printf("Successfully prioritized plan with %d features. Saved to %s\n", len(featureList.Features), outputFile)
		return nil
	},
}

// topologicalSortWithPriority implements Kahn's algorithm with a priority queue
func topologicalSortWithPriority(g *runner.TaskGraph) ([]string, error) {
	// 1. Build In-Degree Map and Adjacency List
	inDegree := make(map[string]int)
	adj := make(map[string][]string)

	for id, node := range g.Nodes {
		inDegree[id] = 0 // Initialize
		_ = node         // prevent unused
	}

	for id, node := range g.Nodes {
		for _, depID := range node.Dependencies {
			if _, ok := g.Nodes[depID]; ok {
				adj[depID] = append(adj[depID], id)
				inDegree[id]++
			} else {
				// Dependency does not exist in the graph (orphaned dependency).
			}
		}
	}

	// 2. Initialize Priority Queue with nodes having inDegree 0
	pq := &priorityQueue{}
	for id, degree := range inDegree {
		if degree == 0 {
			node := g.Nodes[id]
			pq.Push(node)
		}
	}

	var result []string

	// 3. Process Queue
	for pq.Len() > 0 {
		// Get highest priority node
		u := pq.Pop()
		result = append(result, u.ID)

		// Decrease in-degree of neighbors
		for _, vID := range adj[u.ID] {
			inDegree[vID]--
			if inDegree[vID] == 0 {
				vNode := g.Nodes[vID]
				pq.Push(vNode)
			}
		}
	}

	// 4. Check for cycles (if result count < total nodes)
	if len(result) != len(g.Nodes) {
		return nil, fmt.Errorf("cycle detected or graph disconnected in a weird way")
	}

	return result, nil
}

// Simple Priority Queue Helper
type priorityQueue struct {
	items []*runner.TaskNode
}

func (pq *priorityQueue) Len() int { return len(pq.items) }

func (pq *priorityQueue) Push(node *runner.TaskNode) {
	pq.items = append(pq.items, node)
	// Sort after push to keep highest priority at the end (or we can just sort when popping)
	// Let's sort descending so Pop removes from end.
	// High priority string should be > Low priority?
	// We need a helper to compare priorities.
	sort.Slice(pq.items, func(i, j int) bool {
		// We want the LAST item to be the HIGHEST priority (for simple Pop)
		// So Less(i, j) should mean i < j in priority.
		return comparePriority(pq.items[i].Priority, pq.items[j].Priority) < 0
	})
}

func (pq *priorityQueue) Pop() *runner.TaskNode {
	if len(pq.items) == 0 {
		return nil
	}
	n := len(pq.items)
	item := pq.items[n-1]
	pq.items = pq.items[:n-1]
	return item
}

// comparePriority returns:
// 1 if p1 > p2
// 0 if p1 == p2
// -1 if p1 < p2
func comparePriority(p1, p2 string) int {
	val1 := getPriorityValue(p1)
	val2 := getPriorityValue(p2)
	if val1 > val2 {
		return 1
	}
	if val1 < val2 {
		return -1
	}
	return 0
}

func getPriorityValue(p string) int {
	p = strings.ToLower(p)
	switch p {
	case "critical", "production", "high":
		return 3
	case "mvp", "medium", "bug": // Bug is usually medium/high, treat as MVP for now
		return 2
	case "poc", "low", "planning":
		return 1
	default:
		return 0
	}
}
