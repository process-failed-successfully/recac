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
	Use:   "visualize",
	Short: "Visualize the system architecture as a Mermaid graph",
	Long:  `Reads the architecture.yaml file and outputs a Mermaid diagram definition.`,
	RunE:  runArchitectVisualize,
}

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().String("arch", "", "Path to architecture.yaml (defaults to .recac/architecture/architecture.yaml)")
}

func runArchitectVisualize(cmd *cobra.Command, args []string) error {
	archPath, _ := cmd.Flags().GetString("arch")

	if archPath == "" {
		// Default path
		// Check current directory
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		archPath = filepath.Join(cwd, ".recac", "architecture", "architecture.yaml")
	}

	// Read file
	data, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("failed to read architecture file at %s: %w", archPath, err)
	}

	// Unmarshal
	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture file: %w", err)
	}

	// Generate Mermaid
	mermaid := architecture.GenerateMermaid(&arch)

	// Print
	fmt.Println(mermaid)

	return nil
}
