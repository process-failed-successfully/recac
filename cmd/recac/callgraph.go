package main

import (
	"fmt"
	"os"
	"regexp"
	"recac/internal/callgraph"

	"github.com/spf13/cobra"
)

var callgraphCmd = &cobra.Command{
	Use:   "callgraph [path]",
	Short: "Generate a call graph from code",
	Long: `Generates a static call graph by analyzing Go source code.
It identifies function declarations and function calls to build a dependency graph.`,
	RunE: runCallgraph,
}

func init() {
	rootCmd.AddCommand(callgraphCmd)
	callgraphCmd.Flags().StringP("output", "o", "", "Output file path")
	callgraphCmd.Flags().String("focus", "", "Regex to focus on specific functions (shows callers and callees)")
	callgraphCmd.Flags().String("format", "mermaid", "Output format (mermaid, dot)")
	callgraphCmd.Flags().Int("depth", 0, "Depth of graph when using focus (0 = infinite)")
}

func runCallgraph(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	focus, _ := cmd.Flags().GetString("focus")
	format, _ := cmd.Flags().GetString("format")
	outputFile, _ := cmd.Flags().GetString("output")
	depth, _ := cmd.Flags().GetInt("depth")

	// Compile focus regex if provided
	var focusRe *regexp.Regexp
	var err error
	if focus != "" {
		focusRe, err = regexp.Compile(focus)
		if err != nil {
			return fmt.Errorf("invalid focus regex: %w", err)
		}
	}

	// Analyze
	calls, err := callgraph.AnalyzeCalls(root)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Generate Output
	var output string
	if format == "dot" {
		output = callgraph.GenerateDotCallGraph(calls, focusRe, depth)
	} else {
		output = callgraph.GenerateMermaidCallGraph(calls, focusRe, depth)
	}

	// Output
	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Call graph saved to %s\n", outputFile)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), output)
	}

	return nil
}
