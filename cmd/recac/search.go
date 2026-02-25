package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Semantically search the codebase using AI",
	Long: `Search for code based on natural language queries.
Phase 1: Identifies relevant files based on the file tree.
Phase 2: Reads the content of those files to pinpoint the exact code.

Example:
  recac search "user authentication logic"
  recac search "where is the retry mechanism defined?"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSearch,
}

func init() {
	rootCmd.AddCommand(searchCmd)
}

type SearchMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Snippet string `json:"snippet"`
	Reason  string `json:"reason"`
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Gather File List
	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Scanning file tree...")
	files, err := listSearchableFiles(cwd)
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no searchable files found in %s", cwd)
	}

	// 2. Initialize Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-search")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// 3. Phase 1: Identify Relevant Files
	fmt.Fprintf(cmd.OutOrStdout(), "🤖 Identifying relevant files among %d candidates...\n", len(files))

	// Chunk the file list if too large?
	// For now, assume it fits in context (most projects have < 1000 files worth searching).
	// If really large, we'd need multiple passes or a vector DB.
	fileListStr := strings.Join(files, "\n")

	prompt1 := fmt.Sprintf(`You are a Code Search Assistant.
Query: "%s"

Given the following file list, identify up to 5 files that are most likely to contain code related to the query.
Return ONLY a JSON array of strings (file paths).

File List:
%s`, query, fileListStr)

	resp1, err := ag.Send(ctx, prompt1)
	if err != nil {
		return fmt.Errorf("phase 1 agent failed: %w", err)
	}

	var relevantFiles []string
	cleanResp1 := utils.CleanJSONBlock(resp1)
	if err := json.Unmarshal([]byte(cleanResp1), &relevantFiles); err != nil {
		// Fallback: try to split by newline if JSON fails
		lines := strings.Split(cleanResp1, "\n")
		for _, l := range lines {
			l = strings.TrimSpace(l)
			l = strings.Trim(l, `",[] `) // naive cleanup
			if l != "" {
				relevantFiles = append(relevantFiles, l)
			}
		}
		if len(relevantFiles) == 0 {
			return fmt.Errorf("failed to parse relevant files from agent: %v", err)
		}
	}

	if len(relevantFiles) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No relevant files identified.")
		return nil
	}

	// Filter out files that don't exist (agent hallucination check)
	var validFiles []string
	for _, f := range relevantFiles {
		if _, err := os.Stat(f); err == nil {
			validFiles = append(validFiles, f)
		}
	}
	relevantFiles = validFiles

	if len(relevantFiles) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Agent identified files that do not exist.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Checking %d files: %v\n", len(relevantFiles), relevantFiles)

	// 4. Phase 2: Pinpoint Code
	var contentBuilder strings.Builder
	for _, f := range relevantFiles {
		content, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to read %s: %v\n", f, err)
			continue
		}
		contentBuilder.WriteString(fmt.Sprintf("File: %s\n'''\n%s\n'''\n\n", f, string(content)))
	}

	prompt2 := fmt.Sprintf(`You are a Code Search Assistant.
Query: "%s"

Analyze the following file contents. Identify the exact code segments that match the query.
Return ONLY a JSON array of objects with the following structure:
[
  {
    "file": "path/to/file.go",
    "line": <line_number>,
    "snippet": "<short_code_snippet>",
    "reason": "<why this matches>"
  }
]

File Contents:
%s`, query, contentBuilder.String())

	fmt.Fprintln(cmd.OutOrStdout(), "🧠 Analyzing content...")
	resp2, err := ag.Send(ctx, prompt2)
	if err != nil {
		return fmt.Errorf("phase 2 agent failed: %w", err)
	}

	var matches []SearchMatch
	cleanResp2 := utils.CleanJSONBlock(resp2)
	if err := json.Unmarshal([]byte(cleanResp2), &matches); err != nil {
		return fmt.Errorf("failed to parse matches: %v\nResponse: %s", err, resp2)
	}

	// 5. Output
	if len(matches) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No matches found in the identified files.")
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nFound Matches:")
	for _, m := range matches {
		fmt.Fprintf(cmd.OutOrStdout(), "\n📄 %s:%d\n", m.File, m.Line)
		fmt.Fprintf(cmd.OutOrStdout(), "Reason: %s\n", m.Reason)
		fmt.Fprintln(cmd.OutOrStdout(), "```")
		fmt.Fprintln(cmd.OutOrStdout(), m.Snippet)
		fmt.Fprintln(cmd.OutOrStdout(), "```")
	}

	return nil
}

func listSearchableFiles(root string) ([]string, error) {
	var files []string
	ignoreMap := DefaultIgnoreMap()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			if ignoreMap[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip binaries and large files?
		if info.Size() > 1024*1024 { // Skip > 1MB
			return nil
		}

		// Use relative path
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}

		// Basic check for text file (naive)
		ext := filepath.Ext(path)
		if ext == "" || ext == ".go" || ext == ".js" || ext == ".ts" || ext == ".py" || ext == ".md" || ext == ".yaml" || ext == ".json" {
			files = append(files, rel)
		}
		return nil
	})

	sort.Strings(files)
	return files, err
}
