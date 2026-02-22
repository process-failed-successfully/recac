package main

import (
	"fmt"
	"os"
	"path/filepath"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var architectVisualizeCmd = &cobra.Command{
	Use:   "visualize [directory]",
	Short: "Visualize the system architecture using Mermaid",
	Long:  `Generates a Mermaid diagram from the architecture.yaml file.
If a directory is provided, it looks for architecture.yaml inside it.
Otherwise, it defaults to .recac/architecture/architecture.yaml.`,
	RunE: runArchitectVisualize,
}

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().StringP("file", "f", "", "Path to architecture.yaml file (overrides directory argument)")
	architectVisualizeCmd.Flags().StringP("out", "o", "", "Output file for Mermaid diagram (default: stdout)")
}

func runArchitectVisualize(cmd *cobra.Command, args []string) error {
	filePath, _ := cmd.Flags().GetString("file")
	outPath, _ := cmd.Flags().GetString("out")

	// Resolve file path
	if filePath == "" {
		dir := ".recac/architecture"
		if len(args) > 0 {
			dir = args[0]
		}
		filePath = filepath.Join(dir, "architecture.yaml")
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read architecture file at %s: %w", filePath, err)
	}

	// Parse YAML
	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture YAML: %w", err)
	}

	// Generate Mermaid
	mermaid := architecture.GenerateMermaid(&arch)

	// Output
	if outPath != "" {
		if err := os.WriteFile(outPath, []byte(mermaid), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Mermaid diagram written to %s\n", outPath)
	} else {
		fmt.Println(mermaid)
	}

	return nil
}
