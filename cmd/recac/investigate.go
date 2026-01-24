package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	investigateLimit int
	investigateSince string
	investigatePath  string
)

var investigateCmd = &cobra.Command{
	Use:   "investigate [symptom]",
	Short: "Investigate the root cause of a symptom using AI and Git history",
	Long: `Analyzes recent git commits to find the likely cause of a reported symptom.
It fetches recent commits, extracts their diffs, and asks the AI agent to correlate the changes with the symptom.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runInvestigate,
}

func init() {
	rootCmd.AddCommand(investigateCmd)
	investigateCmd.Flags().IntVarP(&investigateLimit, "limit", "n", 5, "Number of recent commits to analyze")
	investigateCmd.Flags().StringVarP(&investigateSince, "since", "s", "1 day ago", "Look for commits since duration (e.g. '2 days ago', '24 hours ago')")
	investigateCmd.Flags().StringVarP(&investigatePath, "path", "p", "", "Limit analysis to a specific file or directory")
}

type InvestigationResult struct {
	SHA       string `json:"sha"`
	Author    string `json:"author"`
	Message   string `json:"message"`
	Score     int    `json:"score"`
	Reasoning string `json:"reasoning"`
}

func runInvestigate(cmd *cobra.Command, args []string) error {
	symptom := strings.Join(args, " ")

	// 1. Setup Context and Clients
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	client := gitClientFactory()
	if !client.RepoExists(cwd) {
		return fmt.Errorf("current directory is not a git repository")
	}

	// 2. Fetch Commits
	// git log --pretty=format:"%H|%an|%s" --since="..." -n ... [path]
	logArgs := []string{
		"--pretty=format:%H|%an|%s",
		fmt.Sprintf("-n%d", investigateLimit),
	}

	// Add since if provided (Git is smart about parsing time strings)
	if investigateSince != "" {
		logArgs = append(logArgs, fmt.Sprintf("--since=%s", investigateSince))
	}

	if investigatePath != "" {
		logArgs = append(logArgs, "--", investigatePath)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Fetching commits since '%s'...\n", investigateSince)
	logs, err := client.Log(cwd, logArgs...)
	if err != nil {
		return fmt.Errorf("git log failed: %w", err)
	}

	if len(logs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No commits found in the specified range.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d commits. Analyzing...\n", len(logs))

	// 3. Initialize Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-investigate")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	var results []InvestigationResult

	// 4. Analyze each commit
	for i, line := range logs {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}
		sha := parts[0]
		author := parts[1]
		msg := parts[2]

		fmt.Fprintf(cmd.OutOrStdout(), "[%d/%d] Analyzing %s: %s\n", i+1, len(logs), sha[:7], msg)

		// Get Diff
		// Diff(sha^, sha)
		diff, err := client.Diff(cwd, sha+"^", sha)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not get diff for %s: %v\n", sha, err)
			continue
		}

		// Truncate diff if too long
		if len(diff) > 10000 {
			diff = diff[:10000] + "\n... (truncated)"
		}

		prompt := fmt.Sprintf(`You are an expert Detective.
Analyze the following commit to determine if it is the cause of the reported symptom.

Symptom: "%s"

Commit: %s
Author: %s
Message: %s

Diff:
'''
%s
'''

Task:
1. Rate the probability (0-10) that this commit caused the symptom.
2. Explain your reasoning.

Return the response in JSON format only:
{
  "score": <0-10 integer>,
  "reasoning": "<short explanation>"
}
`, symptom, sha, author, msg, diff)

		resp, err := ag.Send(ctx, prompt)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Agent failed: %v\n", err)
			continue
		}

		// Clean JSON response (strip markdown blocks if any)
		jsonStr := utils.CleanCodeBlock(resp)

		var res InvestigationResult
		if err := json.Unmarshal([]byte(jsonStr), &res); err != nil {
			// Fallback: try to just print the response if parsing fails?
			// Or just set score to -1
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to parse agent response: %v\n", err)
			res.Reasoning = resp // Store raw response as reasoning
			res.Score = 0
		} else {
			res.SHA = sha
			res.Author = author
			res.Message = msg
		}
		results = append(results, res)
	}

	// 5. Sort and Display Results
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score // Descending
	})

	fmt.Fprintln(cmd.OutOrStdout(), "\n📊 Investigation Results:")
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SCORE\tCOMMIT\tAUTHOR\tREASONING")
	for _, r := range results {
		// Color code score
		scoreDisplay := fmt.Sprintf("%d/10", r.Score)

		// Truncate reasoning
		reason := r.Reasoning
		if len(reason) > 60 {
			reason = reason[:57] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", scoreDisplay, r.SHA[:7], r.Author, reason)
	}
	w.Flush()

	// 6. Recommendation
	if len(results) > 0 && results[0].Score >= 7 {
		fmt.Fprintf(cmd.OutOrStdout(), "\n🤖 Recommendation: Check commit %s by %s.\n", results[0].SHA[:7], results[0].Author)
		fmt.Fprintf(cmd.OutOrStdout(), "Reason: %s\n", results[0].Reasoning)
	}

	return nil
}
