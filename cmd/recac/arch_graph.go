package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var archGraphCmd = &cobra.Command{
	Use:   "graph [path]",
	Short: "Visualize the architectural layers and dependencies",
	Long:  `Generates a Mermaid graph showing the relationships between defined architectural layers.
Edges are colored green for allowed dependencies and red for violations.`,
	RunE: runArchGraph,
}

func init() {
	// archCmd is defined in arch.go (package main)
	archCmd.AddCommand(archGraphCmd)
}

func runArchGraph(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if len(args) > 0 {
		cwd = args[0]
	}

	// 1. Load Config
	// archConfigPath is defined in arch.go
	config, err := loadArchConfig(archConfigPath, cwd)
	if err != nil {
		return fmt.Errorf("failed to load arch config: %w", err)
	}

	// 2. Compile Regexes
	layerRegexps := make(map[string]*regexp.Regexp)
	for name, pattern := range config.Layers {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid regex for layer '%s': %w", name, err)
		}
		layerRegexps[name] = re
	}

	// 3. Analyze Dependencies
	moduleName, err := getModuleName(cwd)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not read go.mod: %v\n", err)
		moduleName = "unknown"
	}

	deps, err := analyzeDependencies(cwd, moduleName, nil, false)
	if err != nil {
		return fmt.Errorf("dependency analysis failed: %w", err)
	}

	// 4. Aggregate Dependencies (Layer -> Layer)
	edges := make(map[string]map[string]int)

	// Helper to identify layer
	getLayer := func(pkg string) string {
		// To ensure deterministic assignment if regexes overlap, we should sort keys.
		// But map iteration is random.
		// Let's sort layer names.
		var names []string
		for k := range layerRegexps {
			names = append(names, k)
		}
		sort.Strings(names)

		for _, name := range names {
			if layerRegexps[name].MatchString(pkg) {
				return name
			}
		}
		return ""
	}

	for srcPkg, targets := range deps {
		srcLayer := getLayer(srcPkg)
		if srcLayer == "" {
			continue
		}

		for _, tgtPkg := range targets {
			tgtLayer := getLayer(tgtPkg)
			if tgtLayer == "" {
				continue
			}
			if srcLayer == tgtLayer {
				continue
			}

			if edges[srcLayer] == nil {
				edges[srcLayer] = make(map[string]int)
			}
			edges[srcLayer][tgtLayer]++
		}
	}

	// 5. Build Mermaid
	fmt.Fprintln(cmd.OutOrStdout(), generateMermaidArchGraph(edges, config))

	return nil
}

func generateMermaidArchGraph(edges map[string]map[string]int, config *ArchConfig) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Define Nodes (Layers)
	var layers []string
	for l := range config.Layers {
		layers = append(layers, l)
	}
	sort.Strings(layers)

	for _, l := range layers {
		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", l, l))
	}

	// Allowed map
	allowed := make(map[string]map[string]bool)
	for _, rule := range config.Rules {
		if allowed[rule.From] == nil {
			allowed[rule.From] = make(map[string]bool)
		}
		for _, allow := range rule.Allow {
			allowed[rule.From][allow] = true
		}
	}

	// Flatten edges for sorting
	type Edge struct {
		From, To string
		Count    int
		Valid    bool
	}
	var flatEdges []Edge

	for src, targets := range edges {
		for tgt, count := range targets {
			valid := false
			if allowed[src] != nil && allowed[src][tgt] {
				valid = true
			}
			flatEdges = append(flatEdges, Edge{src, tgt, count, valid})
		}
	}

	sort.Slice(flatEdges, func(i, j int) bool {
		if flatEdges[i].From != flatEdges[j].From {
			return flatEdges[i].From < flatEdges[j].From
		}
		return flatEdges[i].To < flatEdges[j].To
	})

	edgeIndex := 0
	for _, e := range flatEdges {
		sb.WriteString(fmt.Sprintf("    %s --> %s\n", e.From, e.To))

		// Color the edge
		if !e.Valid {
			sb.WriteString(fmt.Sprintf("    linkStyle %d stroke:#ff0000,stroke-width:2px,color:red;\n", edgeIndex))
		} else {
			sb.WriteString(fmt.Sprintf("    linkStyle %d stroke:#00ff00,stroke-width:2px;\n", edgeIndex))
		}
		edgeIndex++
	}

	// Add legend? Maybe too much text.

	return sb.String()
}
