package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	podcastSince  string
	podcastOutput string
	podcastSpeak  bool
)

var podcastCmd = &cobra.Command{
	Use:   "podcast",
	Short: "Generate an AI-hosted podcast about your codebase",
	Long: `Generates a script for a podcast episode summarizing recent changes, code health, and project status.
Featuring two AI hosts: "The Enthusiast" (Optimistic) and "The Skeptic" (Pragmatic).

If --speak is enabled, it attempts to read the script aloud using 'espeak' or 'say'.`,
	RunE: runPodcast,
}

func init() {
	rootCmd.AddCommand(podcastCmd)
	podcastCmd.Flags().StringVar(&podcastSince, "since", "7d", "Time window to analyze")
	podcastCmd.Flags().StringVarP(&podcastOutput, "output", "o", "", "Output file path for the script")
	podcastCmd.Flags().BoolVar(&podcastSpeak, "speak", false, "Read the script aloud (requires 'espeak' or 'say')")
}

func runPodcast(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Read flags from command to allow testing override
	since, _ := cmd.Flags().GetString("since")
	output, _ := cmd.Flags().GetString("output")
	speak, _ := cmd.Flags().GetBool("speak")

	// 1. Gather Data
	fmt.Fprintln(cmd.OutOrStdout(), "🎙️  Gathering material for the show...")

	// Git History
	// use parseDurationExtended from debt.go to handle "d" suffix
	sinceArg := since
	if d, err := parseDurationExtended(since); err == nil {
		sinceArg = time.Now().Add(-d).Format(time.RFC3339)
	} else {
		// Fallback: if parse fails, assume user passed something git understands directly.
	}

	commits, err := getCatchupCommits(cwd, sinceArg, "", "", nil)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to fetch git history: %v\n", err)
		commits = []string{"(No git history available)"}
	}

	// Code Health (Reusing runAudit logic)
	auditRes, err := runAudit(cwd)
	auditScore := "N/A"
	auditSummary := "Audit failed"
	if err == nil && auditRes != nil {
		auditScore = fmt.Sprintf("%d/100", auditRes.Score)
		auditSummary = fmt.Sprintf("Complexity: %.2f, Duplication: %d blocks, TODOs: %d",
			auditRes.Complexity.Average, auditRes.Duplication.Blocks, auditRes.Todos.Count)
	}

	// Project Context (README)
	readme := "(No README found)"
	if content, err := os.ReadFile("README.md"); err == nil {
		// Truncate to avoid huge context
		if len(content) > 5000 {
			readme = string(content[:5000]) + "\n...(truncated)"
		} else {
			readme = string(content)
		}
	}

	// 2. Construct Prompt
	prompt := fmt.Sprintf(`You are the producer of "The Daily Commit", a tech podcast about this software project.
Generate a script for a 3-5 minute episode featuring two hosts:
- **Alex (The Enthusiast)**: Excited about new features, optimistic, loves clean code.
- **Sam (The Skeptic)**: Pragmatic, worried about technical debt, security, and complexity.

Based on the following data, generate a dialogue script.
They should discuss:
1. The project overview (from README).
2. Recent changes (from Git History).
3. The current Code Health Score (%s) and stats (%s).

Data:
---
README Snippet:
%s

Recent Commits (last %s):
%s
---

Format the output as a script:
**Alex**: ...
**Sam**: ...

Keep it engaging, professional but conversational. Do not use sound effects notation.
`, auditScore, auditSummary, readme, since, strings.Join(commits, "\n"))

	// 3. Call Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-podcast")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🎧 Recording episode (generating script)...")

	script, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 4. Output
	if output != "" {
		if err := os.WriteFile(output, []byte(script), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Script saved to %s\n", output)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "\n"+script)
	}

	// 5. Speak (Optional)
	if speak {
		fmt.Fprintln(cmd.OutOrStdout(), "\n🔊 Playing audio...")
		// Clean markdown for TTS
		cleanScript := cleanForTTS(script)
		if err := speakScript(cleanScript); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to play audio: %v\n", err)
		}
	}

	return nil
}

func speakScript(script string) error {
	// Use execCommand for testability
	// Try 'say' (macOS) first
	if _, err := execCommand("which", "say").CombinedOutput(); err == nil {
		cmd := execCommand("say")
		cmd.Stdin = strings.NewReader(script)
		return cmd.Run()
	}

	// Try 'espeak' (Linux)
	if _, err := execCommand("which", "espeak").CombinedOutput(); err == nil {
		cmd := execCommand("espeak")
		cmd.Stdin = strings.NewReader(script)
		return cmd.Run()
	}

	return fmt.Errorf("neither 'say' nor 'espeak' found in PATH")
}

func cleanForTTS(text string) string {
	// Remove markdown bold/italic (** or __ or *)
	re := regexp.MustCompile(`[\*\_]{1,2}`)
	text = re.ReplaceAllString(text, "")

	// Remove Headers (#) at start of line (entire line)
	reHead := regexp.MustCompile(`(?m)^#+.*$`)
	text = reHead.ReplaceAllString(text, "")

	return text
}
