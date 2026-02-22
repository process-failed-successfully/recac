package main

import (
	"fmt"
	"os"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var architectVisualizeCmd = &cobra.Command{
	Use:   "visualize [path]",
	Short: "Generate Mermaid diagram for architecture.yaml",
	Long:  `Reads an architecture.yaml file and outputs a Mermaid graph definition.
If no path is provided, it defaults to .recac/architecture/architecture.yaml.`,
	RunE: runArchitectVisualize,
}

var visualizeOut string

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().StringVarP(&visualizeOut, "out", "o", "", "Output file path (default: stdout)")
}

func runArchitectVisualize(cmd *cobra.Command, args []string) error {
	var archPath string
	if len(args) > 0 {
		archPath = args[0]
	} else {
		// Default path
		archPath = ".recac/architecture/architecture.yaml"
		if _, err := os.Stat(archPath); os.IsNotExist(err) {
			// Try just architecture.yaml in current dir
			if _, err := os.Stat("architecture.yaml"); err == nil {
				archPath = "architecture.yaml"
			}
		}
	}

	// Read file
	data, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("failed to read architecture file '%s': %w", archPath, err)
	}

	// Parse YAML
	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Generate Mermaid
	mermaid := architecture.GenerateMermaid(&arch)

	// Output
	if visualizeOut != "" {
		if err := os.WriteFile(visualizeOut, []byte(mermaid), 0644); err != nil {
			return fmt.Errorf("failed to write output to '%s': %w", visualizeOut, err)
		}
		fmt.Printf("✅ Mermaid diagram written to %s\n", visualizeOut)
	} else {
		fmt.Println(mermaid)
	}

	return nil
}
