package main

import (
	"fmt"
	"recac/internal/cmdutils"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var jiraListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List Jira tickets",
	Long:    "List Jira tickets in a table format. Use --jql to filter.",
	RunE:    runJiraList,
}

func init() {
	jiraCmd.AddCommand(jiraListCmd)
	jiraListCmd.Flags().String("jql", "assignee = currentUser() ORDER BY updated DESC", "JQL query to filter tickets")
}

func runJiraList(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client, err := cmdutils.GetJiraClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create Jira client: %w", err)
	}

	jql, _ := cmd.Flags().GetString("jql")
	if jql == "" {
		jql = "assignee = currentUser() ORDER BY updated DESC"
	}

	// Fetch tickets
	issues, err := client.SearchIssues(ctx, jql)
	if err != nil {
		return fmt.Errorf("failed to search issues: %w", err)
	}

	if len(issues) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No tickets found.")
		return nil
	}

	// Print table
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "KEY\tTYPE\tSTATUS\tSUMMARY")

	for _, issue := range issues {
		key, _ := issue["key"].(string)
		fields, ok := issue["fields"].(map[string]interface{})
		if !ok {
			continue
		}

		summary, _ := fields["summary"].(string)

		var statusName string
		if statusMap, ok := fields["status"].(map[string]interface{}); ok {
			statusName, _ = statusMap["name"].(string)
		}

		var typeName string
		if typeMap, ok := fields["issuetype"].(map[string]interface{}); ok {
			typeName, _ = typeMap["name"].(string)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", key, typeName, statusName, summary)
	}

	return w.Flush()
}
