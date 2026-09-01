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
	Long:  `Generates a Mermaid graph definition from an architecture.yaml file.`,
	RunE:  runVisualizeCmd,
}

func init() {
	architectCmd.AddCommand(visualizeCmd)
	visualizeCmd.Flags().String("out", "", "Output file path (default: stdout)")
}

func runVisualizeCmd(cmd *cobra.Command, args []string) error {
	// 1. Resolve Path
	// If arg provided, use it.
	// If arg is dir, append "architecture.yaml".
	// If no arg, check .recac/architecture/architecture.yaml
	var path string
	if len(args) > 0 {
		info, err := os.Stat(args[0])
		if err == nil && info.IsDir() {
			path = filepath.Join(args[0], "architecture.yaml")
		} else {
			path = args[0]
		}
	} else {
		// Default location
		path = ".recac/architecture/architecture.yaml"
	}

	// 2. Read File
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read architecture file at %s: %w", path, err)
	}

	// 3. Parse YAML
	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// 4. Generate Mermaid
	mermaid := architecture.GenerateMermaid(&arch)

	// 5. Output
	outFile, _ := cmd.Flags().GetString("out")
	if outFile != "" {
		if err := os.WriteFile(outFile, []byte(mermaid), 0644); err != nil {
			return fmt.Errorf("failed to write output to %s: %w", outFile, err)
		}
		fmt.Printf("✅ Mermaid diagram generated at %s\n", outFile)
	} else {
		fmt.Println(mermaid)
	}

	return nil
}
