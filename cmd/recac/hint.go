package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var hintCmd = newHintCmd()

func newHintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hint",
		Short: "Displays a cheatsheet of common recac commands.",
		Long:  `Provides a quick reference to the most common and useful recac commands, complete with examples.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), getHintText())
		},
	}
	return cmd
}

func getHintText() string {
	var builder strings.Builder

	builder.WriteString("recac CLI Cheatsheet\n")
	builder.WriteString("====================\n\n")

	builder.WriteString("📄 Session Management\n")
	builder.WriteString("  - recac start:         Start a new interactive session.\n")
	builder.WriteString("    (e.g., recac start --goal \"Refactor the authentication module\")\n")
	builder.WriteString("  - recac ls:            List all active and archived sessions.\n")
	builder.WriteString("  - recac ps:            Show running processes for all sessions.\n")
	builder.WriteString("  - recac attach [id]:   Attach to a running session's TUI.\n")
	builder.WriteString("  - recac stop [id]:     Stop a running session.\n")
	builder.WriteString("  - recac rm [id]:       Delete a session.\n\n")

	builder.WriteString("🔍 Inspection & Debugging\n")
	builder.WriteString("  - recac logs [id]:         View the logs for a specific session.\n")
	builder.WriteString("  - recac inspect [id]:      Show detailed metadata for a session.\n")
	builder.WriteString("  - recac status:            Display a real-time dashboard of all sessions.\n")
	builder.WriteString("  - recac doctor:            Run diagnostic checks on your environment.\n\n")

	builder.WriteString("💡 General Usage\n")
	builder.WriteString("  - recac --help:        Show the full list of commands.\n")
	builder.WriteString("  - recac [cmd] --help:  Show help for a specific command.\n")
	builder.WriteString("  - recac hint:          Display this cheatsheet.\n")

	return builder.String()
}

func initHintCmd(rootCmd *cobra.Command) {
	rootCmd.AddCommand(hintCmd)
}
