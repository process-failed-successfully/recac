package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"recac/internal/cmdutils"
	"recac/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var boardCmd = &cobra.Command{
	Use:   "board",
	Short: "Interactive Kanban board for Jira tickets",
	Long:  `View and manage Jira tickets in an interactive Kanban board. Select a ticket to start a coding session.`,
	RunE:  runBoard,
}

func init() {
	rootCmd.AddCommand(boardCmd)
	boardCmd.Flags().String("jql", "", "Custom JQL query (default: assignee = currentUser())")
}

func runBoard(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	client, err := cmdutils.GetJiraClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create Jira client: %w", err)
	}

	jql, _ := cmd.Flags().GetString("jql")
	if jql == "" {
		jql = "assignee = currentUser() ORDER BY updated DESC"
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Fetching tickets with JQL: %s\n", jql)
	issues, err := client.SearchIssues(ctx, jql)
	if err != nil {
		return fmt.Errorf("failed to search issues: %w", err)
	}

	if len(issues) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No tickets found.")
		return nil
	}

	var todos, inProgress, dones []ui.TicketItem

	for _, issue := range issues {
		key, _ := issue["key"].(string)
		fields, ok := issue["fields"].(map[string]interface{})
		if !ok {
			continue
		}

		summary, _ := fields["summary"].(string)

		// Safe description extraction
		desc := "No description"
		if d, ok := fields["description"].(string); ok {
			desc = d
		}

		statusMap, _ := fields["status"].(map[string]interface{})
		statusName, _ := statusMap["name"].(string)
		statusCategoryMap, _ := statusMap["statusCategory"].(map[string]interface{})
		statusCategory, _ := statusCategoryMap["name"].(string) // "To Do", "In Progress", "Done"

		item := ui.TicketItem{
			ID:      key,
			Summary: summary,
			Desc:    desc,
			Status:  statusName,
		}

		// Group logic
		// Jira Status Categories are usually "To Do" (grey), "In Progress" (blue), "Done" (green)
		// We map them to our 3 columns.

		// Normalize
		normCat := strings.ToLower(statusCategory)
		normStatus := strings.ToLower(statusName)

		if strings.Contains(normCat, "done") || strings.Contains(normStatus, "closed") || strings.Contains(normStatus, "resolved") {
			dones = append(dones, item)
		} else if strings.Contains(normCat, "in progress") || strings.Contains(normStatus, "progress") {
			inProgress = append(inProgress, item)
		} else {
			// Default to ToDo
			todos = append(todos, item)
		}
	}

	// Run TUI
	// Test Hook: Skip TUI if requested
	if os.Getenv("RECAC_TEST_SKIP_TUI") == "1" {
		fmt.Fprintf(cmd.OutOrStdout(), "Board initialized with %d To Do, %d In Progress, %d Done\n", len(todos), len(inProgress), len(dones))
		return nil
	}

	// Need to handle full screen or alternate screen? bubbletea usually handles it.
	p := tea.NewProgram(ui.NewBoardModel(todos, inProgress, dones), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI failed: %w", err)
	}

	finalModel, ok := m.(ui.BoardModel)
	if !ok {
		return fmt.Errorf("failed to get board model")
	}

	if finalModel.SelectedTicket != nil {
		t := finalModel.SelectedTicket
		fmt.Fprintf(cmd.OutOrStdout(), "Selected Ticket: %s\n", t.Title())

		// Ask to start session?
		// We can just exec it.
		fmt.Fprintf(cmd.OutOrStdout(), "Starting session for %s...\n", t.ID)

		// We construct the command
		// recac start --jira ID

		// Since we are inside the binary, we can't easily exec "recac" if it's not in PATH.
		// Use os.Executable
		exe, err := os.Executable()
		if err != nil {
			exe = "recac"
		}

		runCmd := exec.Command(exe, "start", "--jira", t.ID)
		runCmd.Stdin = os.Stdin
		runCmd.Stdout = os.Stdout
		runCmd.Stderr = os.Stderr

		return runCmd.Run()
	}

	return nil
}
