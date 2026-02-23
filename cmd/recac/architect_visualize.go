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
	Short: "Visualize the system architecture",
	Long:  "Generates a Mermaid diagram from the architecture.yaml file.",
	RunE:  runArchitectVisualize,
}

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().StringP("file", "f", ".recac/architecture/architecture.yaml", "Path to architecture.yaml file")
	architectVisualizeCmd.Flags().StringP("out", "o", "", "Output file (default is stdout)")
}

func runArchitectVisualize(cmd *cobra.Command, args []string) error {
	filePath, _ := cmd.Flags().GetString("file")
	outPath, _ := cmd.Flags().GetString("out")

	// If filePath is a directory, append architecture.yaml
	info, err := os.Stat(filePath)
	if err == nil && info.IsDir() {
		filePath = filepath.Join(filePath, "architecture.yaml")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read architecture file: %w", err)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture file: %w", err)
	}

	mermaid := architecture.GenerateMermaid(&arch)

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
