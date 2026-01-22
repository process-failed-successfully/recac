package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	catchupSince  string
	catchupTopic  string
	catchupAuthor string
	catchupFiles  []string
	catchupOutput string
)

var catchupCmd = &cobra.Command{
	Use:   "catchup",
	Short: "Generate a topic-focused digest of recent changes",
	Long: `Generates an AI-summarized digest of what happened in the codebase over a specified period, optionally filtered by topic, author, or files.

Useful for:
- catching up after time off
- tracking changes to a specific feature (topic)
- reviewing a colleague's recent work`,
	Example: `  recac catchup --since 7d
  recac catchup --topic "auth" --since 30d
  recac catchup --author "jules" --output report.md`,
	RunE: runCatchup,
}

func init() {
	rootCmd.AddCommand(catchupCmd)
	catchupCmd.Flags().StringVar(&catchupSince, "since", "24h", "Time window to analyze (e.g. 24h, 7d)")
	catchupCmd.Flags().StringVar(&catchupTopic, "topic", "", "Filter commits by topic (searches commit messages)")
	catchupCmd.Flags().StringVar(&catchupAuthor, "author", "", "Filter commits by author")
	catchupCmd.Flags().StringSliceVar(&catchupFiles, "files", nil, "Filter commits touching specific files/paths")
	catchupCmd.Flags().StringVarP(&catchupOutput, "output", "o", "", "Output file path")
}

func runCatchup(cmd *cobra.Command, args []string) error {
	// Parse duration to validate, but git log takes relative dates nicely like "1 week ago" or "2023-01-01"
	// However, user might pass "7d", which git log understands as "7 days ago" if formatted right?
	// Actually git log --since accepts "1 week", "2 days", "yesterday".
	// But let's support Go duration strings for consistency with other commands if possible,
	// or just pass raw string to git if it fails parsing?
	// To be safe, let's try to convert Go duration to a date if it parses.

	sinceArg := catchupSince
	if d, err := time.ParseDuration(catchupSince); err == nil {
		// Convert duration to absolute time from now to ensure consistency
		sinceArg = time.Now().Add(-d).Format(time.RFC3339)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// 1. Fetch Git Log
	commits, err := getCatchupCommits(cwd, sinceArg, catchupTopic, catchupAuthor, catchupFiles)
	if err != nil {
		return fmt.Errorf("failed to fetch git history: %w", err)
	}

	if len(commits) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No matching commits found.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d commits matching criteria. Generating digest...\n", len(commits))

	// 2. Generate Prompt
	prompt := buildCatchupPrompt(commits, catchupSince, catchupTopic)

	// 3. Call Agent
	ctx := context.Background()
	// Reuse agentClientFactory from main package
	agent, err := agentClientFactory(ctx, viper.GetString("provider"), viper.GetString("model"), cwd, "catchup")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	resp, err := agent.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 4. Output
	if catchupOutput != "" {
		if err := os.WriteFile(catchupOutput, []byte(resp), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Digest saved to %s\n", catchupOutput)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "\n"+resp)
	}

	return nil
}

func getCatchupCommits(root, since, topic, author string, files []string) ([]string, error) {
	// git log --since=... --format="%h|%an|%ad|%s|%b" --name-status
	// We want enough info for the agent.

	args := []string{"log", fmt.Sprintf("--since=%s", since), "--format=COMMIT::%h|%an|%ad|%s|%b"}

	if topic != "" {
		args = append(args, "--grep="+topic, "--regexp-ignore-case")
	}

	if author != "" {
		args = append(args, "--author="+author)
	}

	// File pathspecs go after "--"
	if len(files) > 0 {
		args = append(args, "--")
		args = append(args, files...)
	}

	cmd := execCommand("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		// If git fails, it might be no commits or not a repo
		// Check error?
		if exitError, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git log failed: %s", string(exitError.Stderr))
		}
		return nil, err
	}

	output := string(out)
	if strings.TrimSpace(output) == "" {
		return nil, nil
	}

	// Split by "COMMIT::" marker to handle multi-line bodies safely
	rawCommits := strings.Split(output, "COMMIT::")
	var cleaned []string
	for _, c := range rawCommits {
		if strings.TrimSpace(c) == "" {
			continue
		}
		cleaned = append(cleaned, strings.TrimSpace(c))
	}

	return cleaned, nil
}

func buildCatchupPrompt(commits []string, since, topic string) string {
	var sb strings.Builder

	sb.WriteString("You are a technical lead creating a development digest.\n")
	sb.WriteString(fmt.Sprintf("Summarize the following git history from the last %s", since))
	if topic != "" {
		sb.WriteString(fmt.Sprintf(" related to the topic '%s'", topic))
	}
	sb.WriteString(".\n\n")

	sb.WriteString("Focus on:\n")
	sb.WriteString("1. Key architectural changes or decisions.\n")
	sb.WriteString("2. New features or capabilities added.\n")
	sb.WriteString("3. Significant refactors or deletions.\n")
	sb.WriteString("4. Potential impact/risks introduced.\n")
	sb.WriteString("\nFormat the output as a clean Markdown report with sections.\n\n")

	sb.WriteString("## Git History\n")
	// Limit input size if too huge?
	// For now, let's just dump it. If it exceeds context, the agent client might handle it or fail.
	// We could truncate if needed.

	const maxChars = 50000
	currentChars := 0

	for _, c := range commits {
		if currentChars+len(c) > maxChars {
			sb.WriteString("\n... (truncated history) ...\n")
			break
		}
		sb.WriteString(c)
		sb.WriteString("\n---\n")
		currentChars += len(c) + 5
	}

	return sb.String()
}
