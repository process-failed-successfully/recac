package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"recac/internal/agent"
	"recac/internal/runner"
)

var specFile string
var forceInit bool

func init() {
	initProjectCmd.Flags().StringVar(&specFile, "spec", "app_spec.txt", "Path to application specification file")
	initProjectCmd.Flags().BoolVar(&forceInit, "force", false, "Overwrite existing files")
	initProjectCmd.Flags().Bool("mock-agent", false, "Use mock agent for testing")
	viper.BindPFlag("mock-agent", initProjectCmd.Flags().Lookup("mock-agent"))
	
	rootCmd.AddCommand(initProjectCmd)
}

type CLIMockAgent struct {

	Response string

}



func (m *CLIMockAgent) Send(ctx context.Context, prompt string) (string, error) {

	return m.Response, nil

}



var initProjectCmd = &cobra.Command{

	Use:   "init-project",

	Short: "Initialize a new project structure",

	Long:  `Scaffolds a new project based on the application specification. Generates feature_list.json and creates directory structure.`,

	Run: func(cmd *cobra.Command, args []string) {

		fmt.Printf("Initializing project from spec: %s\n", specFile)



		// 1. Read Spec

		specContent, err := os.ReadFile(specFile)

		if err != nil {

			fmt.Printf("Error reading spec file: %v\n", err)

			os.Exit(1)

		}



		// 2. Initialize Agent

		var a agent.Agent

		if viper.GetBool("mock-agent") {

			fmt.Println("Using Mock Agent...")

			a = &CLIMockAgent{

				Response: `[{"category":"core","description":"Initial Feature","steps":["Step 1"],"passes":false}]`,

			}

		} else {


			apiKey := os.Getenv("GEMINI_API_KEY")
			if apiKey == "" {
				fmt.Println("Error: GEMINI_API_KEY is required.")
				os.Exit(1)
			}
			a = agent.NewGeminiClient(apiKey, "gemini-pro")
		}

		// 3. Generate Feature List
		fmt.Println("Generating feature list (this may take a moment)...")
		features, err := runner.GenerateFeatureList(context.Background(), a, string(specContent))
		if err != nil {
			fmt.Printf("Error generating feature list: %v\n", err)
			os.Exit(1)
		}

		// 4. Save Feature List
		if _, err := os.Stat("feature_list.json"); err == nil && !forceInit {
			fmt.Println("feature_list.json already exists. Use --force to overwrite.")
		} else {
			data, _ := json.MarshalIndent(features, "", "  ")
			if err := os.WriteFile("feature_list.json", data, 0644); err != nil {
				fmt.Printf("Error writing feature_list.json: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Created feature_list.json")
		}

		// 5. Create Directory Structure
		dirs := []string{
			"cmd",
			"internal/agent",
			"internal/runner",
			"internal/ui",
			"pkg",
			"scripts",
			"docs",
		}

		for _, dir := range dirs {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Printf("Error creating directory %s: %v\n", dir, err)
			} else {
				// Create .gitkeep to ensure git tracks it
				os.WriteFile(filepath.Join(dir, ".gitkeep"), []byte(""), 0644)
			}
		}
		fmt.Println("Project structure created successfully.")
	},
}
