package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/agent"
	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	learnOutput  string
	learnPersona string
	learnLimit   int
)

var learnCmd = &cobra.Command{
	Use:   "learn",
	Short: "Learn project context and create a persona",
	Long: `Analyzes the codebase, git history, and key files to "learn" the project's coding style, conventions, and terminology.
Generates a project context file (default: .recac/project_context.md) and creates/updates a custom AI persona (default: "project").`,
	RunE: runLearn,
}

func init() {
	rootCmd.AddCommand(learnCmd)
	learnCmd.Flags().StringVarP(&learnOutput, "output", "o", ".recac/project_context.md", "Output file for the learned context")
	learnCmd.Flags().StringVarP(&learnPersona, "persona", "p", "project", "Name of the persona to create/update")
	learnCmd.Flags().IntVarP(&learnLimit, "limit", "l", 5, "Number of source files to sample for analysis")
}

func runLearn(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Gather Context
	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing project structure and history...")

	// Git History
	gitClient := gitClientFactory()
	var gitHistory string
	if gitClient.RepoExists(cwd) {
		logs, err := gitClient.Log(cwd, "--pretty=format:%h %an: %s", "-n", "10")
		if err == nil {
			gitHistory = strings.Join(logs, "\n")
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to get git logs: %v\n", err)
		}
	} else {
		fmt.Fprintln(cmd.ErrOrStderr(), "Warning: not a git repository, skipping history analysis.")
	}

	// File Tree
	// Use generateContextFunc from factories.go which points to GenerateCodebaseContext
	opts := ContextOptions{
		Roots:   []string{"."},
		MaxSize: 50 * 1024, // 50KB limit for initial scan
		Tree:    true,
		// Limit file content scanning here, we will do targeted sampling later
		NoContent: true,
	}

	fileTree, err := generateContextFunc(opts)
	if err != nil {
		return fmt.Errorf("failed to generate file tree: %w", err)
	}

	// Read Key Files
	keyFiles := []string{"README.md", "CONTRIBUTING.md", "go.mod", "package.json", "requirements.txt", "pom.xml", "build.gradle"}
	var keyFileContents strings.Builder
	for _, f := range keyFiles {
		if content, err := os.ReadFile(f); err == nil {
			keyFileContents.WriteString(fmt.Sprintf("File: %s\n```\n%s\n```\n\n", f, string(content)))
		}
	}

	// Sample Source Files
	// Naive approach: list files, filter by common extensions, pick top N by size or random?
	// Let's reuse listSearchableFiles from search.go if accessible, or implement simple logic.
	// Since listSearchableFiles is in search.go (package main), we can use it.
	allFiles, err := listSearchableFiles(cwd)
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	// Filter and select sample files (e.g., largest .go files, or just first N)
	// For simplicity, let's pick first N valid source files.
	var sampleFiles []string
	count := 0
	for _, f := range allFiles {
		if count >= learnLimit {
			break
		}
		// Skip key files already read
		isKey := false
		for _, kf := range keyFiles {
			if f == kf {
				isKey = true
				break
			}
		}
		if isKey {
			continue
		}

		sampleFiles = append(sampleFiles, f)
		count++
	}

	var sampleContent strings.Builder
	for _, f := range sampleFiles {
		if content, err := os.ReadFile(f); err == nil {
			// Truncate if too large
			if len(content) > 10*1024 {
				content = content[:10*1024]
			}
			sampleContent.WriteString(fmt.Sprintf("File: %s\n```\n%s\n```\n\n", f, string(content)))
		}
	}

	// 2. Construct Prompt
	prompt := fmt.Sprintf(`You are an expert software architect and technical writer.
Your goal is to "learn" this project to help future AI agents work more effectively on it.

Analyze the following project context:
1. Git History (Recent changes and commit style)
2. File Tree (Project structure)
3. Key Files (README, Configs)
4. Sample Source Code (Coding style, patterns)

Generate two things:
1. A **Project Context Summary** (Markdown) describing the purpose, architecture, tech stack, and key conventions.
2. A **System Prompt** for an AI "Persona" that embodies this project's style and requirements.

Return a JSON object with the following structure:
{
  "context_summary": "Markdown content...",
  "system_prompt": "You are an expert developer working on [Project Name]..."
}

Input Context:

Git History:
%s

File Tree:
%s

Key Files:
%s

Sample Code:
%s`, gitHistory, fileTree, keyFileContents.String(), sampleContent.String())

	// 3. Call Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-learn")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🧠 Analyzing project context with AI...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed to analyze project: %w", err)
	}

	// 4. Parse Response
	cleanResp := utils.CleanJSONBlock(resp)
	var result struct {
		ContextSummary string `json:"context_summary"`
		SystemPrompt   string `json:"system_prompt"`
	}
	if err := json.Unmarshal([]byte(cleanResp), &result); err != nil {
		return fmt.Errorf("failed to parse agent response: %w\nResponse: %s", err, resp)
	}

	// 5. Save Output
	// Create output dir if needed
	if err := os.MkdirAll(filepath.Dir(learnOutput), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := os.WriteFile(learnOutput, []byte(result.ContextSummary), 0644); err != nil {
		return fmt.Errorf("failed to write context file: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "✅ Project context saved to %s\n", learnOutput)

	// 6. Save Persona
	pm := agent.NewPersonaManager()
	if err := pm.LoadPersonas(); err != nil {
		return fmt.Errorf("failed to load personas: %w", err)
	}

	newPersona := agent.Persona{
		Name:         fmt.Sprintf("Project: %s", filepath.Base(cwd)),
		Description:  fmt.Sprintf("Auto-generated persona for %s project.", filepath.Base(cwd)),
		SystemPrompt: result.SystemPrompt,
	}

	pm.AddPersona(learnPersona, newPersona)
	if err := pm.SavePersonas(); err != nil {
		return fmt.Errorf("failed to save persona: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "✅ Persona '%s' created/updated.\n", learnPersona)

	return nil
}
