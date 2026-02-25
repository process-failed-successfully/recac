package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"recac/internal/analysis"

	"github.com/spf13/cobra"
)

var archGraphCmd = &cobra.Command{
	Use:   "graph [path]",
	Short: "Visualize architectural layers",
	Long: `Generates a high-level dependency graph between architectural layers.
Edges are colored green (allowed) or red (violation).
Requires an architecture config file (see 'recac arch').`,
	RunE: runArchGraph,
}

func init() {
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
	config, err := loadArchConfig(archConfigPath, cwd)
	if err != nil {
		return fmt.Errorf("failed to load arch config: %w", err)
	}

	// 2. Compile Regexes
	layerRegexps := make(map[string]string)
	for name, pattern := range config.Layers {
		layerRegexps[name] = pattern
	}

	// 3. Analyze Dependencies
	moduleName, err := analysis.GetModuleName(cwd)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not read go.mod: %v\n", err)
		moduleName = "unknown"
	}

	opts := analysis.DependencyOptions{
		Root:       cwd,
		ModuleName: moduleName,
		ShowStdLib: false,
	}
	deps, err := analysis.AnalyzeDependencies(opts)
	if err != nil {
		return fmt.Errorf("dependency analysis failed: %w", err)
	}

	// 4. Generate Layer Graph
	regexps := make(map[string]*regexp.Regexp)
	for name, pat := range layerRegexps {
		re, err := regexp.Compile(pat)
		if err == nil {
			regexps[name] = re
		}
	}

	graph := generateLayerGraph(deps, config, regexps)
	fmt.Fprintln(cmd.OutOrStdout(), graph)

	return nil
}

func generateLayerGraph(deps analysis.DepMap, config *ArchConfig, regexps map[string]*regexp.Regexp) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	getLayer := func(pkg string) string {
		for name, re := range regexps {
			if re.MatchString(pkg) {
				return name
			}
		}
		return ""
	}

	// 1. Identify Nodes (Layers)
	// We should include all layers defined in config, even if empty?
	// Or only those with packages?
	// Test expects nodes to exist. Let's add all layers from config.
	var layers []string
	for name := range regexps {
		layers = append(layers, name)
	}
	sort.Strings(layers)

	for _, l := range layers {
		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", l, l))
	}

	// 2. Identify Edges (Layer -> Layer)
	edges := make(map[string]bool) // "src->tgt" -> true

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

			edgeKey := fmt.Sprintf("%s->%s", srcLayer, tgtLayer)
			edges[edgeKey] = true
		}
	}

	// 3. Render Edges & Calculate Styles
	var edgeList []string
	for e := range edges {
		edgeList = append(edgeList, e)
	}
	sort.Strings(edgeList)

	// Build allowed map for checking styles
	allowed := make(map[string]map[string]bool)
	for _, rule := range config.Rules {
		if allowed[rule.From] == nil {
			allowed[rule.From] = make(map[string]bool)
		}
		for _, allow := range rule.Allow {
			allowed[rule.From][allow] = true
		}
	}

	for i, e := range edgeList {
		parts := strings.Split(e, "->")
		src, tgt := parts[0], parts[1]

		sb.WriteString(fmt.Sprintf("    %s --> %s\n", src, tgt))

		// Check if allowed
		isAllowed := false
		if allowedMap, ok := allowed[src]; ok {
			if allowedMap[tgt] {
				isAllowed = true
			}
		}

		color := "#ff0000" // Red (Violation)
		if isAllowed {
			color = "#00ff00" // Green (Allowed)
		}

		// Mermaid linkStyle is 0-indexed based on order of links
		sb.WriteString(fmt.Sprintf("    linkStyle %d stroke:%s,stroke-width:2px;\n", i, color))
	}

	return sb.String()
}
