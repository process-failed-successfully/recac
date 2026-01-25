package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"recac/internal/ui"

	"github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// blameExecCommand allows mocking
var blameExecCommand = exec.Command

var blameCmd = &cobra.Command{
	Use:   "blame [file]",
	Short: "Interactive AI-powered git blame",
	Long:  `Explore file history with an interactive TUI. View blame information line-by-line, see diffs, and ask AI to explain changes.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runBlame,
}

func init() {
	rootCmd.AddCommand(blameCmd)
}

func runBlame(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	// Verify file exists
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// 1. Run git blame
	output, err := executeGitBlame(filePath)
	if err != nil {
		return fmt.Errorf("git blame failed: %w", err)
	}

	// 2. Parse output
	lines, err := parseBlameOutput(output)
	if err != nil {
		return fmt.Errorf("failed to parse blame output: %w", err)
	}

	// 3. Setup Callbacks
	ctx := context.Background()
	cwd, _ := os.Getwd()

	fetchDiff := func(sha string) (string, error) {
		// Use git show for the commit
		c := blameExecCommand("git", "show", sha)
		out, err := c.CombinedOutput()
		return string(out), err
	}

	explainFunc := func(sha string) (string, error) {
		provider := viper.GetString("provider")
		model := viper.GetString("model")
		ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-blame")
		if err != nil {
			return "", err
		}

		// Get Diff for context
		diff, _ := fetchDiff(sha)
		if len(diff) > 2000 {
			diff = diff[:2000] + "\n...(truncated)"
		}

		prompt := fmt.Sprintf(`Explain the following commit in the context of the file history.
Commit: %s
Diff:
'''
%s
'''

Explain WHY this change was likely made and what it does. Be concise.`, sha, diff)

		return ag.Send(ctx, prompt)
	}

	// 4. Run TUI
	m := ui.NewBlameModel(lines, fetchDiff, explainFunc)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running blame TUI: %w", err)
	}

	return nil
}

func executeGitBlame(file string) ([]byte, error) {
	cmd := blameExecCommand("git", "blame", "--line-porcelain", "--", file)
	return cmd.Output()
}

func parseBlameOutput(data []byte) ([]ui.BlameLine, error) {
	var lines []ui.BlameLine
	scanner := bufio.NewScanner(bytes.NewReader(data))

	current := ui.BlameLine{}
	lineNo := 0

	// --line-porcelain format:
	// <sha> <orig_line> <final_line> <num_lines>
	// author <name>
	// ...
	// filename <name>
	// \t<content>

	for scanner.Scan() {
		text := scanner.Text()

		if strings.HasPrefix(text, "\t") {
			// This is the content line, ends the block
			current.Content = strings.TrimPrefix(text, "\t")
			current.LineNo = lineNo + 1
			lines = append(lines, current)
			lineNo++
			current = ui.BlameLine{} // Reset
			continue
		}

		// Parse Headers
		parts := strings.SplitN(text, " ", 2)
		if len(parts) < 2 {
			continue // Should not happen in porcelain usually, maybe boundary
		}

		key := parts[0]
		value := parts[1]

		// SHA is at the start of a new block, but how do we detect it versus a header?
		// Headers keys are known (author, committer, summary, filename, boundary, previous, etc)
		// SHA line is 40 chars hex (or less) + numbers.
		// AND it is the first line of the block.
		// But since we reset `current` after content, we know we are at start.

		if current.SHA == "" {
			// This must be the SHA line
			// Format: sha orig_line final_line [count]
			shaParts := strings.Fields(text)
			if len(shaParts) >= 3 {
				current.SHA = shaParts[0]
			}
			continue
		}

		switch key {
		case "author":
			current.Author = value
		case "author-time":
			if ts, err := strconv.ParseInt(value, 10, 64); err == nil {
				t := time.Unix(ts, 0)
				current.Date = t.Format("2006-01-02")
			}
		case "summary":
			current.Summary = value
		}
	}

	return lines, scanner.Err()
}
