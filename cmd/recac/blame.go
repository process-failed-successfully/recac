package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var blameCmd = &cobra.Command{
	Use:   "blame [file] [line]",
	Short: "Explain the history of a line of code using AI",
	Long: `Analyze the git blame history of a specific line of code and use AI to explain
the context, the commit message, and the diff that introduced the change.

If no line number is provided, it will list the file with line numbers and prompt you to select one.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runBlame,
}

func init() {
	rootCmd.AddCommand(blameCmd)
}

func runBlame(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	var line int
	var err error

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filePath)
	}

	git := gitClientFactory()
	cwd, _ := os.Getwd()

	if len(args) == 2 {
		line, err = strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid line number: %s", args[1])
		}
	} else {
		// No line number provided, show file with line numbers
		// We use "cat -n" style or read file and print with line numbers
		lines, err := utils.ReadLines(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "File content:")
		for i, l := range lines {
			fmt.Fprintf(cmd.OutOrStdout(), "%4d: %s\n", i+1, l)
		}

		fmt.Fprint(cmd.OutOrStdout(), "\nEnter line number to blame: ")
		var lineStr string
		fmt.Fscanln(cmd.InOrStdin(), &lineStr)
		line, err = strconv.Atoi(lineStr)
		if err != nil {
			return fmt.Errorf("invalid line number")
		}
	}

	// 1. Get Blame Info
	// git blame -L <line>,<line> --porcelain <file>
	// The --porcelain format is easier to parse but for simplicity, let's just get the hash first
	// standard output: <hash> (<author> <date> <line>) <content>
	// We use -L to limit to one line
	blameArgs := []string{"blame", "-L", fmt.Sprintf("%d,%d", line, line), "--porcelain", filePath}
	blameOut, err := git.Run(cwd, blameArgs...)
	if err != nil {
		return fmt.Errorf("git blame failed: %w", err)
	}

	// Parse porcelain output
	// First line is hash
	lines := strings.Split(blameOut, "\n")
	if len(lines) == 0 {
		return fmt.Errorf("empty blame output")
	}
	parts := strings.Fields(lines[0])
	if len(parts) == 0 {
		return fmt.Errorf("invalid blame output")
	}
	commitHash := parts[0]

	// Get full commit info
	// git show <hash>
	showOut, err := git.Run(cwd, "show", commitHash)
	if err != nil {
		return fmt.Errorf("git show failed: %w", err)
	}

	// 2. Ask AI
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-blame")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Read the actual line content for context
	fileLines, _ := utils.ReadLines(filePath)
	lineContent := ""
	if line > 0 && line <= len(fileLines) {
		lineContent = fileLines[line-1]
	}

	prompt := fmt.Sprintf(`You are an expert code historian.
I am looking at line %d of %s:
'''
%s
'''

It was last modified in commit %s.
Here is the full commit details (message and diff):
'''
%s
'''

Please explain:
1. Who changed this line and when?
2. What was the purpose of the commit (based on the message)?
3. How does the diff relate to this specific line?
4. Why does the code look like this now?

Be concise and helpful.`, line, filePath, lineContent, commitHash, showOut)

	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing history with AI...")
	fmt.Fprintln(cmd.OutOrStdout(), "")

	_, err = ag.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
	})
	fmt.Fprintln(cmd.OutOrStdout(), "")

	return err
}
