package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var standupCmd = &cobra.Command{
	Use:   "standup",
	Short: "Generate a daily standup report using AI",
	Long:  `Aggregates git activity, completed agent sessions, and TODOs to generate a daily standup report via AI.`,
	RunE:  runStandup,
}

func init() {
	standupCmd.Flags().String("since", "24h", "Time window for the report (e.g. 12h, 24h)")
	standupCmd.Flags().StringP("output", "o", "", "Output file path")
	rootCmd.AddCommand(standupCmd)
}

func runStandup(cmd *cobra.Command, args []string) error {
	sinceStr, _ := cmd.Flags().GetString("since")
	duration, err := time.ParseDuration(sinceStr)
	if err != nil {
		return fmt.Errorf("invalid duration format: %w", err)
	}

	sinceTime := time.Now().Add(-duration)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// 1. Git Activity
	gitClient := gitClientFactory()
	logs, err := gitClient.Log(cwd, "--since="+sinceStr, "--pretty=format:%h|%an|%s|%ad")
	if err != nil {
		// Non-fatal, maybe not a git repo
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not fetch git logs: %v\n", err)
	}

	// 2. Session Activity
	sm, err := sessionManagerFactory()
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}
	sessions, err := sm.ListSessions()
	if err != nil {
		// Non-fatal, maybe no sessions
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not list sessions: %v\n", err)
	}

	var recentSessions []string
	if sessions != nil {
		for _, s := range sessions {
			// check if session was active in the window
			active := false
			if s.StartTime.After(sinceTime) {
				active = true
			} else if !s.EndTime.IsZero() && s.EndTime.After(sinceTime) {
				active = true
			} else if s.Status == "running" {
				active = true
			}

			if active {
				status := s.Status
				if status == "" {
					status = "unknown"
				}
				recentSessions = append(recentSessions, fmt.Sprintf("- %s: %s (Status: %s)", s.Name, s.Goal, status))
			}
		}
	}

	// 3. TODOs
	// Scan code todos
	codeTodos, _ := ScanForTodos(cwd)
	// Read TODO.md
	todoMdStats := "No TODO.md found"
	if _, err := os.Stat("TODO.md"); err == nil {
		lines, _ := utils.ReadLines("TODO.md")
		done, pending := 0, 0
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "- [x]") {
				done++
			} else if strings.HasPrefix(strings.TrimSpace(line), "- [ ]") {
				pending++
			}
		}
		todoMdStats = fmt.Sprintf("Pending: %d, Completed: %d", pending, done)
	}

	// 4. Construct Prompt
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Generate a Daily Standup Report covering the last %s.\n\n", sinceStr))

	sb.WriteString("## Git Activity\n")
	if len(logs) == 0 {
		sb.WriteString("No commits found.\n")
	} else {
		for _, l := range logs {
			sb.WriteString(fmt.Sprintf("- %s\n", l))
		}
	}

	sb.WriteString("\n## Agent Sessions\n")
	if len(recentSessions) == 0 {
		sb.WriteString("No active sessions.\n")
	} else {
		for _, s := range recentSessions {
			sb.WriteString(fmt.Sprintf("%s\n", s))
		}
	}

	sb.WriteString("\n## Task Status\n")
	sb.WriteString(fmt.Sprintf("TODO.md: %s\n", todoMdStats))
	sb.WriteString(fmt.Sprintf("Codebase TODOs: %d detected\n", len(codeTodos)))

	sb.WriteString("\nInstructions:\n")
	sb.WriteString("- Summarize the work done based on commits and sessions.\n")
	sb.WriteString("- Highlight any blockers or failures (failed sessions).\n")
	sb.WriteString("- Mention technical debt if code TODO count is high.\n")
	sb.WriteString("- Keep it professional and concise.\n")

	// 5. Call Agent
	ctx := context.Background()
	agent, err := agentClientFactory(ctx, viper.GetString("provider"), viper.GetString("model"), cwd, "standup")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🤖 Generating standup report for last %s...\n", sinceStr)

	resp, err := agent.Send(ctx, sb.String())
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 6. Output
	outputFile, _ := cmd.Flags().GetString("output")
	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(resp), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Report saved to %s\n", outputFile)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "\n"+resp)
	}

	return nil
}
