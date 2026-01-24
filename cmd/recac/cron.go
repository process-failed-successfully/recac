package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"recac/internal/utils"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cronExplain string
	cronNext    int
)

var cronCmd = &cobra.Command{
	Use:   "cron [description]",
	Short: "Generate, explain, and verify cron expressions",
	Long: `Generate cron expressions from natural language descriptions,
explain existing cron expressions, and verify them by showing next run times.

Examples:
  recac cron "every 5 minutes"
  recac cron "at 2am on Sundays"
  recac cron --explain "*/5 * * * *"`,
	RunE: runCron,
}

func init() {
	rootCmd.AddCommand(cronCmd)
	cronCmd.Flags().StringVarP(&cronExplain, "explain", "e", "", "Cron expression to explain")
	cronCmd.Flags().IntVarP(&cronNext, "next", "n", 5, "Number of next execution times to show")
}

func runCron(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-cron")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Mode 1: Explain
	if cronExplain != "" {
		fmt.Fprintln(cmd.OutOrStdout(), "🤖 Analyzing cron expression...")
		prompt := fmt.Sprintf("Explain the following cron expression in simple terms:\n```\n%s\n```\nProvide a breakdown of the schedule.", cronExplain)
		resp, err := ag.Send(ctx, prompt)
		if err != nil {
			return fmt.Errorf("agent failed: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), resp)

		// Also show next runs for verification
		showNextRuns(cmd, cronExplain)
		return nil
	}

	// Mode 2: Generate
	if len(args) == 0 {
		return fmt.Errorf("please provide a description or use --explain")
	}
	description := strings.Join(args, " ")

	fmt.Fprintf(cmd.OutOrStdout(), "Generating cron for: %s\n", description)

	prompt := fmt.Sprintf(`You are a cron expression expert.
Generate a standard cron expression (5 fields: Minute Hour Dom Month Dow) for the following requirement:
"%s"

Requirements:
1. Return ONLY the raw cron expression. Do not wrap it in markdown code blocks.
2. Do not add explanations.
3. Use standard 5-field format (Minute, Hour, Day of Month, Month, Day of Week).
`, description)

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Consulting AI...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// Clean up response
	candidate := utils.CleanCodeBlock(resp)
	candidate = strings.TrimSpace(candidate)
	// Sometimes agents put it in backticks or similar, utils.CleanCodeBlock handles triple backticks but maybe inline ones too?
	candidate = strings.Trim(candidate, "`")

	fmt.Fprintf(cmd.OutOrStdout(), "\nResult: %s\n", candidate)

	// Verify and show next runs
	showNextRuns(cmd, candidate)

	return nil
}

func showNextRuns(cmd *cobra.Command, spec string) {
	// Try standard parser first
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(spec)
	if err != nil {
		// Try with seconds?
		parserSeconds := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		scheduleSeconds, err2 := parserSeconds.Parse(spec)
		if err2 != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  Could not parse cron expression: %v\n", err)
			return
		}
		schedule = scheduleSeconds
		fmt.Fprintln(cmd.OutOrStdout(), "(Parsed as 6-field cron with seconds)")
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nNext scheduled runs:")
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	now := time.Now()
	for i := 0; i < cronNext; i++ {
		next := schedule.Next(now)
		if next.IsZero() {
			break
		}
		fmt.Fprintf(w, "%d.\t%s\t(%s)\n", i+1, next.Format(time.RFC1123), time.Until(next).Round(time.Second))
		now = next
	}
	w.Flush()
}
