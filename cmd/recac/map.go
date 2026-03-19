package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"recac/internal/analysis"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var mapCmd = &cobra.Command{
	Use:   "map [path]",
	Short: "Visualize code architecture",
	Long: `Generates a dependency graph of the Go packages in the project.
Can output in Mermaid (default) or DOT format.
Use --explain to have the AI analyze the architecture.`,
	RunE: runMap,
}

func init() {
	rootCmd.AddCommand(mapCmd)
	initMapFlags(mapCmd)
}

func initMapFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("output", "o", "", "Output file path")
	cmd.Flags().StringP("format", "f", "mermaid", "Output format (mermaid, dot)")
	cmd.Flags().Bool("explain", false, "Use AI to explain the architecture")
	cmd.Flags().StringSliceP("ignore", "i", []string{}, "Ignore packages matching regex")
	cmd.Flags().String("focus", "", "Focus on a specific package (substring)")
	cmd.Flags().Bool("stdlib", false, "Include standard library dependencies")
}

func runMap(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	// Read flags
	mapOutput, _ := cmd.Flags().GetString("output")
	mapFormat, _ := cmd.Flags().GetString("format")
	mapExplain, _ := cmd.Flags().GetBool("explain")
	mapIgnore, _ := cmd.Flags().GetStringSlice("ignore")
	// Clean up pflag artifacts
	var cleanIgnore []string
	for _, p := range mapIgnore {
		if p != "[]" {
			cleanIgnore = append(cleanIgnore, p)
		}
	}
	mapIgnore = cleanIgnore

	mapFocus, _ := cmd.Flags().GetString("focus")
	mapShowStdLib, _ := cmd.Flags().GetBool("stdlib")

	// 1. Determine Module Name
	moduleName, err := analysis.GetModuleName(root)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not read go.mod: %v\n", err)
	}

	// 2. Analyze Dependencies
	opts := analysis.DependencyOptions{
		Root:           root,
		ModuleName:     moduleName,
		IgnorePatterns: mapIgnore,
		ShowStdLib:     mapShowStdLib,
	}

	deps, err := analysis.AnalyzeDependencies(opts)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// 4. Generate Output
	var output string
	switch strings.ToLower(mapFormat) {
	case "dot":
		output = generateDOT(deps, moduleName, mapFocus)
	case "mermaid":
		output = generateMermaidMap(deps, moduleName, mapFocus)
	default:
		return fmt.Errorf("unknown format: %s", mapFormat)
	}

	// 5. Handle Output
	if mapOutput != "" {
		if err := os.WriteFile(mapOutput, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Graph saved to %s\n", mapOutput)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), output)
	}

	// 6. Explain if requested
	if mapExplain {
		return explainArchitecture(cmd, output)
	}

	return nil
}

func generateMermaidMap(deps analysis.DepMap, moduleName string, focus string) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Sort keys for deterministic output
	var keys []string
	for k := range deps {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, src := range keys {
		// Filter focus
		if focus != "" && !strings.Contains(src, focus) {
			continue
		}

		safeSrc := sanitizeID(src)

		targets := deps[src]
		sort.Strings(targets)

		for _, tgt := range targets {
			safeTgt := sanitizeID(tgt)

			// If moduleName is known, highlight internal vs external
			if strings.HasPrefix(tgt, moduleName) {
				sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeSrc, safeTgt))
			} else {
				// External dependency style
				sb.WriteString(fmt.Sprintf("    %s -.-> %s\n", safeSrc, safeTgt))
			}
		}
	}

	return sb.String()
}

func generateDOT(deps analysis.DepMap, moduleName string, focus string) string {
	var sb strings.Builder
	sb.WriteString("digraph G {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [shape=box, style=filled, color=\"#dddddd\"];\n")

	var keys []string
	for k := range deps {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, src := range keys {
		// Filter focus
		if focus != "" && !strings.Contains(src, focus) {
			continue
		}

		targets := deps[src]
		sort.Strings(targets)
		for _, tgt := range targets {
			sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\";\n", src, tgt))
		}
	}

	sb.WriteString("}\n")
	return sb.String()
}

func sanitizeID(id string) string {
	// Fast path: if no characters to replace, just return the original string
	if strings.IndexByte(id, '/') == -1 && strings.IndexByte(id, '-') == -1 && strings.IndexByte(id, '.') == -1 {
		return id
	}

	var sb strings.Builder
	sb.Grow(len(id))
	for i := 0; i < len(id); i++ {
		c := id[i]
		if c == '/' || c == '-' || c == '.' {
			sb.WriteByte('_')
		} else {
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

func explainArchitecture(cmd *cobra.Command, graphStr string) error {
	ctx := context.Background()
	cwd, _ := os.Getwd()

	// Create agent
	ag, err := agentClientFactory(ctx, viper.GetString("provider"), viper.GetString("model"), cwd, "recac-map")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`Analyze the following Go dependency graph (Mermaid/DOT) and describe the software architecture.
Identify:
1. Core components and their responsibilities.
2. Potential architectural bottlenecks or circular dependencies (if any).
3. The overall architectural pattern (e.g., Layered, Hexagonal, Monolith).

Graph:
'''
%s
'''`, graphStr)

	fmt.Fprintln(cmd.OutOrStdout(), "\n🤖 Analyzing architecture...")

	_, err = ag.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
	})
	fmt.Println("")

	return err
}
