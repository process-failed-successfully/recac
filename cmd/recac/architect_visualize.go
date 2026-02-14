package main

import (
	"fmt"
	"os"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var visualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Visualize the system architecture as a Mermaid diagram",
	Long:  "Reads an architecture.yaml file and generates a Mermaid flowchart diagram representing the system components and data flow.",
	Run:   runVisualizeCmd,
}

func init() {
	architectCmd.AddCommand(visualizeCmd)
	visualizeCmd.Flags().String("in", ".recac/architecture/architecture.yaml", "Path to input architecture.yaml")
	visualizeCmd.Flags().String("out", "", "Output file path (default: stdout)")
}

func runVisualizeCmd(cmd *cobra.Command, args []string) {
	inPath, _ := cmd.Flags().GetString("in")
	outPath, _ := cmd.Flags().GetString("out")

	// 1. Read Architecture
	data, err := os.ReadFile(inPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading architecture file: %v\n", err)
		os.Exit(1)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing architecture file: %v\n", err)
		os.Exit(1)
	}

	// 2. Generate Diagram
	diagram := architecture.GenerateMermaid(&arch)

	// 3. Output
	if outPath != "" {
		if err := os.WriteFile(outPath, []byte(diagram), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Diagram written to %s\n", outPath)
	} else {
		fmt.Println(diagram)
	}
}
