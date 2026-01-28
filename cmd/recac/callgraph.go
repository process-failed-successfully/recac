package main

import (
	"fmt"
	"os"
	"recac/internal/analysis"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	callGraphFocus string
	callGraphDir   string
)

var callGraphCmd = &cobra.Command{
	Use:   "callgraph",
	Short: "Generate a static call graph of the codebase",
	Long: `Generates a Mermaid flowchart of function calls.
Useful for understanding code flow and dependencies.
Note: This uses static analysis and heuristics, so it may be approximate.`,
	RunE: runCallGraph,
}

func init() {
	rootCmd.AddCommand(callGraphCmd)
	callGraphCmd.Flags().StringVar(&callGraphFocus, "focus", "", "Focus on a specific function (show callers/callees)")
	callGraphCmd.Flags().StringVar(&callGraphDir, "dir", ".", "Directory to analyze")
}

func runCallGraph(cmd *cobra.Command, args []string) error {
	dir := callGraphDir
	if dir == "." {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	cg, err := analysis.GenerateCallGraph(dir)
	if err != nil {
		return fmt.Errorf("failed to generate call graph: %w", err)
	}

	// Filter if focused
	if callGraphFocus != "" {
		cg = filterGraph(cg, callGraphFocus)
	}

	fmt.Fprintln(cmd.OutOrStdout(), generateMermaidCallGraph(cg))
	return nil
}

func filterGraph(cg *analysis.CallGraph, focus string) *analysis.CallGraph {
	// Find the node(s) matching focus
	// We match partial ID or Name
	relevantNodes := make(map[string]bool)

	for id, node := range cg.Nodes {
		if strings.Contains(strings.ToLower(id), strings.ToLower(focus)) ||
		   strings.Contains(strings.ToLower(node.Name), strings.ToLower(focus)) {
			relevantNodes[id] = true
		}
	}

	if len(relevantNodes) == 0 {
		return cg // Return all or empty? Let's return empty with warning?
		// Actually, returning full graph might be annoying if they expected filter.
		// Let's return empty but check later.
	}

	// Expand to 1 level of depth (callers and callees)
	// Or maybe full connectivity? Let's do 1 level for now to keep it clean.
	expandedNodes := make(map[string]bool)
	for id := range relevantNodes {
		expandedNodes[id] = true
	}

	// Find edges connected to relevant nodes
	var filteredEdges []analysis.CallGraphEdge

	for _, edge := range cg.Edges {
		if relevantNodes[edge.From] || relevantNodes[edge.To] {
			filteredEdges = append(filteredEdges, edge)
			expandedNodes[edge.From] = true
			expandedNodes[edge.To] = true
		}
	}

	// Rebuild nodes map
	filteredNodes := make(map[string]*analysis.CallGraphNode)
	for id := range expandedNodes {
		if node, exists := cg.Nodes[id]; exists {
			filteredNodes[id] = node
		} else {
			// Might be external node
			filteredNodes[id] = &analysis.CallGraphNode{ID: id, Name: id}
		}
	}

	return &analysis.CallGraph{
		Nodes: filteredNodes,
		Edges: filteredEdges,
	}
}

func generateMermaidCallGraph(cg *analysis.CallGraph) string {
	var sb strings.Builder
	sb.WriteString("graph LR\n")

	// Sort nodes for determinism
	var nodeIDs []string
	for id := range cg.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	for _, id := range nodeIDs {
		node := cg.Nodes[id]
		// Sanitize ID for Mermaid
		safeID := SanitizeMermaidID(id)

		// Label: "Pkg.Func" or "(Type).Method"
		label := node.ID
		// Simplify label: remove full path prefix if possible?
		// e.g., "internal/analysis.GenerateCallGraph" -> "analysis.GenerateCallGraph"
		parts := strings.Split(label, "/")
		label = parts[len(parts)-1]

		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", safeID, label))

		// Style based on type
		if strings.Contains(label, "(Ambiguous)") {
			sb.WriteString(fmt.Sprintf("    style %s stroke-dasharray: 5 5\n", safeID))
		}
	}

	// Sort edges
	sort.Slice(cg.Edges, func(i, j int) bool {
		if cg.Edges[i].From != cg.Edges[j].From {
			return cg.Edges[i].From < cg.Edges[j].From
		}
		return cg.Edges[i].To < cg.Edges[j].To
	})

	for _, edge := range cg.Edges {
		safeFrom := SanitizeMermaidID(edge.From)
		safeTo := SanitizeMermaidID(edge.To)

		sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeFrom, safeTo))
	}

	return sb.String()
}
