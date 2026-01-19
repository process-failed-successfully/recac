package main

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var retroCmd = &cobra.Command{
	Use:   "retro [SESSION_ID]",
	Short: "Generate a retrospective report for a session",
	Long:  `Analyzes the logs of a session using the AI agent to generate a retrospective report, highlighting what went well, what failed, and providing actionable insights.`,
	RunE:  runRetro,
}

func init() {
	rootCmd.AddCommand(retroCmd)
}

func runRetro(cmd *cobra.Command, args []string) error {
	sm, err := sessionManagerFactory()
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}

	var sessionName string
	if len(args) > 0 {
		sessionName = args[0]
	} else {
		// Find latest session
		sessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}
		if len(sessions) == 0 {
			return fmt.Errorf("no sessions found")
		}
		// Sort by StartTime descending
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].StartTime.After(sessions[j].StartTime)
		})
		sessionName = sessions[0].Name
		fmt.Fprintf(cmd.ErrOrStderr(), "No session specified. Analyzing latest session: %s\n", sessionName)
	}

	logPath, err := sm.GetSessionLogs(sessionName)
	if err != nil {
		return fmt.Errorf("failed to get logs for session %s: %w", sessionName, err)
	}

	logContent, err := os.ReadFile(logPath)
	if err != nil {
		return fmt.Errorf("failed to read log file %s: %w", logPath, err)
	}

	// Truncate if too long (e.g. 100KB)
	const maxLogSize = 100 * 1024
	logStr := string(logContent)
	if len(logStr) > maxLogSize {
		fmt.Fprintln(cmd.ErrOrStderr(), "Log file too large, truncating to last 100KB...")
		logStr = logStr[len(logStr)-maxLogSize:]
		logStr = "[...TRUNCATED...]\n" + logStr
	}

	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	// Use "recac-retro" as project name/agent ID for context
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-retro")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`Analyze the following session logs for an autonomous coding agent.
Generate a Retrospective Report in Markdown format containing:
1. **Summary**: Brief overview of the session's goal and outcome.
2. **What Went Well**: Successful actions, correct reasoning, efficient steps.
3. **Challenges & Failures**: Errors, loops, misunderstandings, or technical blockers.
4. **Actionable Insights**: Recommendations to improve the agent's prompt, the codebase, or the environment to prevent future issues.

Logs:
'''
%s
'''`, logStr)

	fmt.Fprintln(cmd.ErrOrStderr(), "Generating retrospective report...")
	fmt.Fprintln(cmd.OutOrStdout(), "") // Spacing

	_, err = ag.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
	})
	fmt.Fprintln(cmd.OutOrStdout(), "") // Final newline

	return err
}
