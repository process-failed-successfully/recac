package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"recac/internal/security"
	"recac/internal/ui"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	tea "github.com/charmbracelet/bubbletea"
)

var explorerCmd = &cobra.Command{
	Use:   "explorer [path]",
	Short: "Interactive TUI explorer with AI powers",
	Long:  `Browse your codebase, view file stats, check complexity, scan for secrets, and ask AI to explain code - all from a TUI.`,
	RunE:  runExplorer,
}

func init() {
	rootCmd.AddCommand(explorerCmd)
}

func runExplorer(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	model, err := ui.NewExplorerModel(path, explainAdapter, complexityAdapter, securityAdapter)
	if err != nil {
		return err
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running explorer: %w", err)
	}
	return nil
}

func explainAdapter(path string) (string, error) {
	ctx := context.Background()
	provider := viper.GetString("provider")
	modelName := viper.GetString("model")
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	ag, err := agentClientFactory(ctx, provider, modelName, cwd, "recac-explorer")
	if err != nil {
		return "", fmt.Errorf("failed to create agent: %w", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Truncate content if too large
	strContent := string(content)
	if len(strContent) > 8000 {
		runes := []rune(strContent)
		if len(runes) > 8000 {
			strContent = string(runes[:8000]) + "\n...(truncated)"
		}
	}

	prompt := fmt.Sprintf("Explain this file concisely:\n\n%s", strContent)
	return ag.Send(ctx, prompt)
}

func complexityAdapter(path string) (string, error) {
	// runComplexityAnalysis handles file path correctly
	results, err := runComplexityAnalysis(path)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "No complex functions found (or not a Go file).", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Cyclomatic Complexity Report for %s\n", path))
	sb.WriteString("========================================\n\n")

	for _, r := range results {
		sb.WriteString(fmt.Sprintf("Function: %s\n", r.Function))
		sb.WriteString(fmt.Sprintf("Line: %d\n", r.Line))
		sb.WriteString(fmt.Sprintf("Complexity: %d\n", r.Complexity))
		sb.WriteString("----------------------------------------\n")
	}

	return sb.String(), nil
}

func securityAdapter(path string) (string, error) {
	scanner := security.NewRegexScanner()
	results, err := runSecurityScan(path, scanner)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "✅ No security issues found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Security Findings for %s\n", path))
	sb.WriteString("========================================\n\n")

	for _, r := range results {
		sb.WriteString(fmt.Sprintf("Type: %s\n", r.Type))
		sb.WriteString(fmt.Sprintf("Line: %d\n", r.Line))
		sb.WriteString(fmt.Sprintf("Description: %s\n", r.Description))
		sb.WriteString(fmt.Sprintf("Match: %s\n", r.Match))
		sb.WriteString("----------------------------------------\n")
	}

	return sb.String(), nil
}
