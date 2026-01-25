package main

import (
	"fmt"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	presentationTopic  string
	presentationSince  string
	presentationOutput string
	presentationLimit  int
)

var presentationCmd = &cobra.Command{
	Use:   "presentation",
	Short: "Generate a slide deck from git history",
	Long: `Generates a Markdown slide deck (Marp compatible) summarizing recent work or a specific topic.
It analyzes git history, extracts context, and uses AI to create a structured presentation.

Example:
  recac presentation --since "1 week ago" --output weekly_demo.md
  recac presentation --topic "Refactoring" --limit 10`,
	RunE: runPresentation,
}

func init() {
	rootCmd.AddCommand(presentationCmd)
	presentationCmd.Flags().StringVar(&presentationTopic, "topic", "", "Filter commits by topic (grep)")
	presentationCmd.Flags().StringVar(&presentationSince, "since", "1 week ago", "Look for commits since duration")
	presentationCmd.Flags().StringVarP(&presentationOutput, "output", "o", "presentation.md", "Output file path")
	presentationCmd.Flags().IntVar(&presentationLimit, "limit", 10, "Maximum number of commits to analyze")
}

func runPresentation(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	// 1. Fetch Git Logs
	client := gitClientFactory()
	if !client.RepoExists(cwd) {
		return fmt.Errorf("current directory is not a git repository")
	}

	logArgs := []string{
		"--pretty=format:%H|%an|%s",
		fmt.Sprintf("-n%d", presentationLimit),
	}

	if presentationSince != "" {
		logArgs = append(logArgs, fmt.Sprintf("--since=%s", presentationSince))
	}

	if presentationTopic != "" {
		logArgs = append(logArgs, fmt.Sprintf("--grep=%s", presentationTopic))
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🎥 Fetching commits (Since: %s, Topic: %s)...\n", presentationSince, presentationTopic)
	logs, err := client.Log(cwd, logArgs...)
	if err != nil {
		return fmt.Errorf("git log failed: %w", err)
	}

	if len(logs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No commits found matching criteria.")
		return nil
	}

	// 2. Prepare Context for AI
	var contextBuilder strings.Builder
	contextBuilder.WriteString(fmt.Sprintf("Found %d commits:\n\n", len(logs)))

	for i, line := range logs {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}
		sha := parts[0]
		author := parts[1]
		msg := parts[2]

		contextBuilder.WriteString(fmt.Sprintf("Commit: %s\nAuthor: %s\nMessage: %s\n", sha[:7], author, msg))

		// Fetch a small diff summary for context
		// We use --stat to save token space
		diffStat, err := client.Log(cwd, "-1", "--stat", sha)
		if err == nil && len(diffStat) > 0 {
			// diffStat includes the log message again, we just want the stats usually at the end
			// But for simplicity, let's just ask for diff --stat specifically
			// client.Log wraps git log.
			// Let's try client.Diff with --stat? The interface might not support arbitrary args easily for Diff.
			// Interface: Diff(repoPath, commitA, commitB)
			// Let's rely on Log with --stat which gives: commit info + stat
			// Or just use the message. The message is usually enough for a high level presentation.
			// If we want detailed code, we'd need full diffs which might blow up context.
			// Let's stick to messages for now, maybe fetch full body if needed.
			// Actually, git log --pretty=format only gives subject.
			// Let's get the full body.
			bodyArgs := []string{"-1", "--pretty=format:%b", sha}
			body, _ := client.Log(cwd, bodyArgs...)
			if len(body) > 0 && len(body[0]) > 0 {
				contextBuilder.WriteString(fmt.Sprintf("Details: %s\n", body[0]))
			}
		}
		contextBuilder.WriteString("---\n")

		// Limit processing to avoid token limits if limit is high
		if i >= 15 {
			contextBuilder.WriteString("... (and more)\n")
			break
		}
	}

	// 3. Generate Slides with AI
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-presentation")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are a Developer Advocate.
Create a presentation slide deck based on the following git history.

Target Audience: Technical Team / Stakeholders.
Format: Markdown (Marp compatible).

Guidelines:
- Start with a Title Slide.
- Have an Agenda.
- Group related commits into "Feature" or "Improvement" slides. Don't just list every commit.
- Use bullet points.
- Include a "Next Steps" or "Conclusion" slide.
- Use emoji where appropriate.
- Do NOT include code blocks unless critical. Keep it high-level.

Git History:
'''
%s
'''

Return ONLY the markdown content.
`, contextBuilder.String())

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Generating slides...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 4. Save Output
	cleaned := utils.CleanCodeBlock(resp)

	// Ensure Marp header if missing
	if !strings.Contains(cleaned, "marp: true") {
		cleaned = "---\nmarp: true\ntheme: default\n---\n\n" + cleaned
	}

	if err := os.WriteFile(presentationOutput, []byte(cleaned), 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Presentation saved to %s\n", presentationOutput)
	return nil
}
