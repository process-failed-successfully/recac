package main

import (
	"fmt"
	"os"
	"recac/internal/analysis"

	"github.com/spf13/cobra"
)

var (
	sequenceDepth  int
	sequenceOutput string
	sequenceDir    string
)

var sequenceCmd = &cobra.Command{
	Use:   "sequence [function]",
	Short: "Generate a Mermaid sequence diagram for a function",
	Long:  `Analyzes the Go code to generate a Mermaid sequence diagram tracing the execution flow starting from the specified function.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		entryPoint := args[0]

		fmt.Fprintf(cmd.ErrOrStderr(), "DEBUG: Analyzing %s in %s\n", entryPoint, sequenceDir)
		diagram, err := analysis.GenerateSequence(sequenceDir, entryPoint, sequenceDepth)
		if err != nil {
			return fmt.Errorf("failed to generate sequence diagram: %w", err)
		}

		if sequenceOutput != "" {
			if err := os.WriteFile(sequenceOutput, []byte(diagram), 0644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Diagram saved to %s\n", sequenceOutput)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), diagram)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(sequenceCmd)
	sequenceCmd.Flags().IntVarP(&sequenceDepth, "depth", "d", 5, "Max traversal depth")
	sequenceCmd.Flags().StringVarP(&sequenceOutput, "output", "o", "", "Output file path")
	sequenceCmd.Flags().StringVar(&sequenceDir, "dir", ".", "Directory to analyze")
}
