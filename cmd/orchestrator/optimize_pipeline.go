package main

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"

	"recac/internal/orchestrator"
)

func optimizePipelineJob(filePath, outFile, provider, model string) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to read file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}

	apiKey := ""
	switch provider {
	case "openrouter":
		apiKey = viper.GetString("secrets.openrouter_api_key")
	case "openai":
		apiKey = viper.GetString("secrets.openai_api_key")
	case "anthropic":
		apiKey = viper.GetString("secrets.anthropic_api_key")
	case "gemini":
		apiKey = viper.GetString("secrets.gemini_api_key")
	}

	if apiKey == "" {
		fmt.Fprintf(stdout, "Warning: API key for provider '%s' not found. AI call may fail if not set in environment.\n", provider)
	}

	fmt.Fprintf(stdout, "Optimizing pipeline using AI (%s - %s)...\n", provider, model)

	optimizedYAML, err := orchestrator.OptimizePipelineYAML(context.Background(), string(fileData), provider, model, apiKey)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to optimize pipeline: %v\n", err)
		exitFunc(1)
		return
	}

	if outFile == "" || outFile == "-" {
		// Output to stdout
		style := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			MarginBottom(1)
		fmt.Fprintln(stdout, style.Render("Optimized Pipeline YAML"))
		fmt.Fprintln(stdout, optimizedYAML)
	} else {
		// Write to file
		err = os.WriteFile(outFile, []byte(optimizedYAML), 0644)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to write optimized pipeline to file: %v\n", err)
			exitFunc(1)
			return
		}
		fmt.Fprintf(stdout, "Optimized pipeline successfully written to %s\n", outFile)
	}
}
