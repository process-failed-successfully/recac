package main

import (
	"fmt"
	"os"
	"path/filepath"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var visualizeCmd = &cobra.Command{
	Use:   "visualize [path]",
	Short: "Visualize architecture.yaml as a Mermaid diagram",
	Long: `Generates a Mermaid graph from your architecture definition file.
If no path is provided, it defaults to .recac/architecture/architecture.yaml.
If a directory is provided, it looks for architecture.yaml inside it.`,
	RunE: runVisualize,
}

func init() {
	architectCmd.AddCommand(visualizeCmd)
	visualizeCmd.Flags().StringP("out", "o", "", "Output file path (default stdout)")
}

func runVisualize(cmd *cobra.Command, args []string) error {
	// 1. Resolve Path
	path := ".recac/architecture/architecture.yaml"
	if len(args) > 0 {
		path = args[0]
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("file not found: %s", path)
	}
	if info.IsDir() {
		path = filepath.Join(path, "architecture.yaml")
	}

	// 2. Read File
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// 3. Parse Architecture
	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture.yaml: %w", err)
	}

	// 4. Generate Mermaid
	mermaid := architecture.GenerateMermaid(&arch)

	// 5. Output
	outFile, _ := cmd.Flags().GetString("out")
	if outFile != "" {
		if err := os.WriteFile(outFile, []byte(mermaid), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Printf("Graph saved to %s\n", outFile)
	} else {
		fmt.Println(mermaid)
	}

	return nil
}
