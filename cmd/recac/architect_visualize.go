package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var architectVisualizeCmd = &cobra.Command{
	Use:   "visualize [ARCHITECTURE_PATH]",
	Short: "Visualize the system architecture",
	Long:  "Reads architecture.yaml and generates a Mermaid diagram. Defaults to .recac/architecture/architecture.yaml",
	Args:  cobra.MaximumNArgs(1),
	Run:   runArchitectVisualizeCmd,
}

func init() {
	architectCmd.AddCommand(architectVisualizeCmd)
	architectVisualizeCmd.Flags().String("out", "", "Output file for the diagram (default: stdout)")
	architectVisualizeCmd.Flags().String("format", "mermaid", "Output format (currently only 'mermaid')")
}

func runArchitectVisualizeCmd(cmd *cobra.Command, args []string) {
	// Validate format
	format, _ := cmd.Flags().GetString("format")
	if format != "mermaid" {
		fmt.Fprintf(os.Stderr, "Unsupported format: %s. Only 'mermaid' is supported.\n", format)
		os.Exit(1)
	}

	archPath := filepath.Join(".recac", "architecture", "architecture.yaml")

	if len(args) > 0 {
		inputPath := args[0]
		stat, err := os.Stat(inputPath)
		if err == nil {
			if stat.IsDir() {
				archPath = filepath.Join(inputPath, "architecture.yaml")
			} else {
				archPath = inputPath
			}
		} else {
			// Path doesn't exist or error accessing it.
			// Heuristic: if it ends in .yaml/.yml, assume file. Otherwise assume directory.
			if strings.HasSuffix(inputPath, ".yaml") || strings.HasSuffix(inputPath, ".yml") {
				archPath = inputPath
			} else {
				archPath = filepath.Join(inputPath, "architecture.yaml")
			}
		}
	}

	// 2. Read architecture.yaml
	data, err := os.ReadFile(archPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading architecture file at %s: %v\n", archPath, err)
		os.Exit(1)
	}

	// 3. Parse YAML
	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing architecture YAML: %v\n", err)
		os.Exit(1)
	}

	// 4. Generate Mermaid
	mermaid := architecture.GenerateMermaid(&arch)

	// 5. Output
	outFile, _ := cmd.Flags().GetString("out")
	if outFile != "" {
		if err := os.MkdirAll(filepath.Dir(outFile), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(outFile, []byte(mermaid), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output to %s: %v\n", outFile, err)
			os.Exit(1)
		}
		fmt.Printf("Diagram written to %s\n", outFile)
	} else {
		fmt.Println(mermaid)
	}
}
