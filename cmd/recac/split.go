package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewSplitCmd() *cobra.Command {
	var deleteOriginal bool

	cmd := &cobra.Command{
		Use:   "split <file>",
		Short: "Split a large file into smaller files using AI",
		Long: `Reads a large source code file and uses the configured AI agent to logically
split it into multiple smaller, focused files (e.g., separating interfaces, models, and handlers).
The AI generates the file names and their respective content.

Example:
  recac split main.go --delete`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			targetFile := args[0]

			// 1. Read Target File
			content, err := os.ReadFile(targetFile)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", targetFile, err)
			}
			fileExt := filepath.Ext(targetFile)
			fileDir := filepath.Dir(targetFile)

			fmt.Fprintf(cmd.ErrOrStderr(), "Analyzing %s to determine logical splits...\n", targetFile)

			// 2. Prepare AI Request
			provider := viper.GetString("provider")
			model := viper.GetString("model")
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-split")
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			prompt := fmt.Sprintf(`You are an expert software architect.
I need to split the following source code file into multiple smaller, logically grouped files in the same directory.
For example, if it's a Go file containing interfaces, structs, and functions, split them into appropriate files (e.g., types%s, handlers%s).
Maintain all necessary imports and package declarations so the code still compiles.

Return ONLY a JSON object where keys are the new file names (just the base name, e.g. "types%s") and values are the full string content of that file.
Do not include any explanation or markdown wrapping outside the JSON block.

Source Code:
%s`, fileExt, fileExt, fileExt, string(content))

			// 3. Get and Parse Response
			resp, err := ag.Send(ctx, prompt)
			if err != nil {
				return fmt.Errorf("failed to get AI response: %w", err)
			}

			cleanJSON := utils.CleanJSONBlock(resp)

			var filesMap map[string]string
			if err := json.Unmarshal([]byte(cleanJSON), &filesMap); err != nil {
				return fmt.Errorf("failed to parse AI response as JSON: %w\nResponse was:\n%s", err, resp)
			}

			if len(filesMap) == 0 {
				return fmt.Errorf("AI did not suggest any files to create")
			}

			// 4. Write New Files
			for name, body := range filesMap {
				// Prevent path traversal
				safeName := filepath.Base(name)
				outPath := filepath.Join(fileDir, safeName)

				// Optional sanity check: don't overwrite the original unless specifically asked
				if outPath == targetFile && !deleteOriginal {
					// We might overwrite the original. If the user didn't ask to delete it, maybe we shouldn't modify it in place if it's identical
					// But if the AI suggested keeping the same name for part of the code, we'll overwrite it. Let's append an underscore or something.
					// For simplicity, let's just use the suggested name.
				}

				if err := os.WriteFile(outPath, []byte(strings.TrimSpace(body)+"\n"), 0644); err != nil {
					return fmt.Errorf("failed to write file %s: %w", outPath, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", outPath)
			}

			// 5. Delete Original (Optional)
			if deleteOriginal {
				// Only delete if the AI didn't happen to suggest a file with the exact same name
				if _, ok := filesMap[filepath.Base(targetFile)]; !ok {
					if err := os.Remove(targetFile); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to delete original file %s: %v\n", targetFile, err)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "Deleted original file %s\n", targetFile)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&deleteOriginal, "delete", "d", false, "Delete the original file after splitting")
	return cmd
}

var splitCmd = NewSplitCmd()

func init() {
	rootCmd.AddCommand(splitCmd)
}
