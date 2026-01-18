package main

import (
	"fmt"
	"os"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
)

var (
	ctxCopy      bool
	ctxOutput    string
	ctxTokens    bool
	ctxTree      bool
	ctxMaxSize   int64
	ctxIgnore    []string
	ctxNoContent bool
)

var contextCmd = &cobra.Command{
	Use:   "context [paths...]",
	Short: "Generate a context dump of the codebase for LLMs",
	Long:  `Generates a comprehensive context dump of the specified paths (or current directory) suitable for pasting into an LLM. Includes a file tree and file contents.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		roots := args
		if len(roots) == 0 {
			roots = []string{"."}
		}

		opts := ContextOptions{
			Roots:      roots,
			Ignore:     ctxIgnore,
			MaxSize:    ctxMaxSize,
			NoContent:  ctxNoContent,
			Tree:       ctxTree,
			ShowTokens: ctxTokens,
		}

		result, err := GenerateCodebaseContext(opts)
		if err != nil {
			return err
		}

		// 3. Stats
		if ctxTokens {
			// Rough estimate: 4 chars = 1 token
			tokens := len(result) / 4
			fmt.Fprintf(cmd.ErrOrStderr(), "Estimated Tokens: %d\n", tokens)
		}

		// 4. Output
		if ctxCopy {
			if err := clipboard.WriteAll(result); err != nil {
				// Fallback or warning?
				// On headless systems this might fail.
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to copy to clipboard: %v\n", err)
			} else {
				fmt.Fprintln(cmd.ErrOrStderr(), "Context copied to clipboard!")
			}
		}

		if ctxOutput != "" {
			if err := os.WriteFile(ctxOutput, []byte(result), 0644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Context written to %s\n", ctxOutput)
		}

		if !ctxCopy && ctxOutput == "" {
			fmt.Fprint(cmd.OutOrStdout(), result)
		}

		return nil
	},
}

func init() {
	contextCmd.Flags().BoolVarP(&ctxCopy, "copy", "c", false, "Copy output to clipboard")
	contextCmd.Flags().StringVarP(&ctxOutput, "output", "o", "", "Write output to file")
	contextCmd.Flags().BoolVarP(&ctxTokens, "tokens", "t", false, "Show estimated token count")
	contextCmd.Flags().BoolVarP(&ctxTree, "tree", "T", true, "Include file tree")
	contextCmd.Flags().BoolVar(&ctxNoContent, "no-content", false, "Exclude file contents (tree only)")
	contextCmd.Flags().Int64VarP(&ctxMaxSize, "max-size", "s", 1024*1024, "Max file size to include (bytes)")
	contextCmd.Flags().StringSliceVarP(&ctxIgnore, "ignore", "i", nil, "Additional ignore patterns (directories)")

	rootCmd.AddCommand(contextCmd)
}

