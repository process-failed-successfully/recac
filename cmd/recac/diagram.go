package main

import (
	"fmt"
	"os"
	"regexp"

	"recac/internal/analysis"

	"github.com/spf13/cobra"
)

var diagramCmd = &cobra.Command{
	Use:   "diagram [path]",
	Short: "Generate a class diagram from code",
	Long: `Generates a Mermaid class diagram by analyzing Go structs and their relationships.
This command parses the source code to identify structs, fields, and embeddings.`,
	RunE: runDiagram,
}

func init() {
	rootCmd.AddCommand(diagramCmd)
	diagramCmd.Flags().StringP("output", "o", "", "Output file path")
	diagramCmd.Flags().String("focus", "", "Regex to focus on specific structs by name")
	diagramCmd.Flags().Bool("fields", true, "Include fields in the diagram")
}

func runDiagram(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	focus, _ := cmd.Flags().GetString("focus")
	showFields, _ := cmd.Flags().GetBool("fields")
	outputFile, _ := cmd.Flags().GetString("output")

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
	classes, relationships, err := analysis.AnalyzeStructs(root)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Generate Mermaid
	mermaid := analysis.GenerateMermaidClassDiagram(classes, relationships, focusRe, showFields)

	// Output
	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(mermaid), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Diagram saved to %s\n", outputFile)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), mermaid)
	}

	return nil
}
