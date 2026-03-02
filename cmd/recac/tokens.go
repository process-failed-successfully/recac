package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/agent"
	"recac/internal/utils"

	"github.com/spf13/cobra"
)

var tokensCmd = &cobra.Command{
	Use:     "tokens [path...]",
	Aliases: []string{"count-tokens"},
	Short:   "Estimate the number of AI tokens",
	Long: `Estimates the number of AI tokens in files, directories, or standard input.
It uses an approximate counting method (roughly 4 characters per token).

Examples:
  recac tokens
  recac tokens main.go
  recac count-tokens ./internal/
  cat main.go | recac tokens`,
	RunE: runTokensCmd,
}

func init() {
	rootCmd.AddCommand(tokensCmd)
}

func runTokensCmd(cmd *cobra.Command, args []string) error {
	totalTokens := 0
	filesProcessed := 0

	// Check if stdin has data
	stat, _ := os.Stdin.Stat()
	stdinHasData := (stat.Mode() & os.ModeCharDevice) == 0

	// If stdin has data and no files were explicitly specified
	if stdinHasData && len(args) == 0 {
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}

		tokens := agent.EstimateTokenCount(string(content))
		fmt.Fprintf(cmd.OutOrStdout(), "Estimated tokens (stdin): %d\n", tokens)
		return nil
	}

	// If no args provided and no stdin, use current directory
	if len(args) == 0 && !stdinHasData {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		args = []string{cwd}
	}

	ignoreMap := DefaultIgnoreMap()

	for _, argPath := range args {
		info, err := os.Stat(argPath)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not access path %s: %v\n", argPath, err)
			continue
		}

		if info.IsDir() {
			err = filepath.WalkDir(argPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil // Skip errors
				}

				if d.IsDir() {
					if ignoreMap[d.Name()] {
						return filepath.SkipDir
					}
					if strings.HasPrefix(d.Name(), ".") && d.Name() != "." && d.Name() != ".." {
						return filepath.SkipDir
					}
					return nil
				}

				if ignoreMap[d.Name()] || strings.HasPrefix(d.Name(), ".") {
					return nil
				}

				// Skip binary files
				ext := strings.ToLower(filepath.Ext(path))
				if utils.IsBinaryExt(ext) {
					return nil
				}

				content, err := os.ReadFile(path)
				if err != nil {
					return nil
				}

				tokens := agent.EstimateTokenCount(string(content))
				totalTokens += tokens
				filesProcessed++
				return nil
			})
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: error walking directory %s: %v\n", argPath, err)
			}
		} else {
			// It's a single file
			content, err := os.ReadFile(argPath)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not read file %s: %v\n", argPath, err)
				continue
			}
			tokens := agent.EstimateTokenCount(string(content))
			totalTokens += tokens
			filesProcessed++
		}
	}

	if filesProcessed > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Processed %d files.\n", filesProcessed)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Estimated total tokens: %d\n", totalTokens)

	return nil
}
