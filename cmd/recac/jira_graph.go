package main

import (
	"context"
	"fmt"
	"os"
	"recac/internal/cmdutils"
	"recac/internal/jira"
	"strings"

	"github.com/spf13/cobra"
)

var jiraGraphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Visualize Jira ticket dependencies",
	Long:  `Generates a dependency graph (Mermaid or DOT) for Jira tickets based on "is blocked by" links.`,
	Run:   runJiraGraphCmd,
}

func init() {
	jiraCmd.AddCommand(jiraGraphCmd)
	jiraGraphCmd.Flags().String("project", "", "Filter by project key")
	jiraGraphCmd.Flags().String("label", "", "Filter by label")
	jiraGraphCmd.Flags().String("status", "", "Filter by status (e.g., 'In Progress')")
	jiraGraphCmd.Flags().String("output", "mermaid", "Output format (mermaid, dot)")
}

func runJiraGraphCmd(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	client, err := cmdutils.GetJiraClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	project, _ := cmd.Flags().GetString("project")
	label, _ := cmd.Flags().GetString("label")
	status, _ := cmd.Flags().GetString("status")
	outputFormat, _ := cmd.Flags().GetString("output")

	// Construct JQL
	var conditions []string
	if project != "" {
		conditions = append(conditions, fmt.Sprintf("project = \"%s\"", project))
	}
	if label != "" {
		conditions = append(conditions, fmt.Sprintf("labels = \"%s\"", label))
	}
	if status != "" {
		conditions = append(conditions, fmt.Sprintf("status = \"%s\"", status))
	}

	jql := strings.Join(conditions, " AND ")
	if jql == "" {
		// Default query if no filters provided
		jql = "order by created DESC"
	}

	fmt.Fprintf(os.Stderr, "Fetching issues with JQL: %s\n", jql)
	issues, err := client.SearchIssues(ctx, jql)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching issues: %v\n", err)
		os.Exit(1)
	}

	if len(issues) == 0 {
		fmt.Println("No issues found.")
		return
	}

	// Build Graph
	// We need a map of Key -> Issue details for rendering
	issueMap := make(map[string]map[string]interface{})
	for _, issue := range issues {
		key, _ := issue["key"].(string)
		if key != "" {
			issueMap[key] = issue
		}
	}

	// Use getAllBlockers which returns "KEY (Status)" regardless of status
	// BuildGraphFromIssues parses this format.
	g := jira.BuildGraphFromIssues(issues, getAllBlockers)

	if outputFormat == "dot" {
		fmt.Fprintln(cmd.OutOrStdout(), generateJiraDOT(g, issueMap))
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), generateMermaidJira(g, issueMap))
	}
}

func generateMermaidJira(g *jira.DependencyGraph, issueMap map[string]map[string]interface{}) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Nodes
	for key := range g.AllTickets {
		issue, exists := issueMap[key]
		var summary, status string
		if exists {
			fields, _ := issue["fields"].(map[string]interface{})
			summary, _ = fields["summary"].(string)
			statusMap, _ := fields["status"].(map[string]interface{})
			status, _ = statusMap["name"].(string)
		} else {
			summary = "External Dependency"
			status = "Unknown"
		}

		style := getJiraStyle(status)
		safeID := sanitizeJiraID(key)
		// Clean summary for label
		safeSummary := strings.ReplaceAll(summary, "\"", "'")
		if len(safeSummary) > 30 {
			safeSummary = safeSummary[:27] + "..."
		}

		sb.WriteString(fmt.Sprintf("    %s[\"%s<br/>%s\"]%s\n", safeID, key, safeSummary, style))
	}

	// Edges
	// Blocks: blocker -> blocked
	for blocker, blockedList := range g.Blocks {
		safeBlocker := sanitizeJiraID(blocker)
		for _, blocked := range blockedList {
			safeBlocked := sanitizeJiraID(blocked)
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeBlocker, safeBlocked))
		}
	}

	// Styles
	sb.WriteString("\n    classDef done fill:#90EE90,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef inprogress fill:#87CEEB,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef todo fill:#D3D3D3,stroke:#333,stroke-width:1px,color:black;\n")

	return sb.String()
}

func getJiraStyle(status string) string {
	s := strings.ToLower(status)
	if strings.Contains(s, "done") || strings.Contains(s, "closed") || strings.Contains(s, "resolved") {
		return ":::done"
	}
	if strings.Contains(s, "progress") {
		return ":::inprogress"
	}
	return ":::todo"
}

func sanitizeJiraID(id string) string {
	return strings.ReplaceAll(id, "-", "_")
}

// getAllBlockers returns a list of tickets that block the given ticket, regardless of status.
func getAllBlockers(ticket map[string]interface{}) []string {
	fields, ok := ticket["fields"].(map[string]interface{})
	if !ok {
		return nil
	}

	links, ok := fields["issuelinks"].([]interface{})
	if !ok {
		return nil
	}

	var blockers []string
	for _, link := range links {
		linkMap, ok := link.(map[string]interface{})
		if !ok {
			continue
		}

		linkType, ok := linkMap["type"].(map[string]interface{})
		if !ok {
			continue
		}

		// Look for "is blocked by" relationship (inward)
		inward, _ := linkType["inward"].(string)
		if strings.EqualFold(inward, "is blocked by") {
			inwardIssue, ok := linkMap["inwardIssue"].(map[string]interface{})
			if ok {
				key, _ := inwardIssue["key"].(string)
				fields, _ := inwardIssue["fields"].(map[string]interface{})
				if fields != nil {
					status, _ := fields["status"].(map[string]interface{})
					if status != nil {
						statusName, _ := status["name"].(string)
						// Include all blockers regardless of status
						blockers = append(blockers, fmt.Sprintf("%s (%s)", key, statusName))
					}
				}
			}
		}
	}

	return blockers
}

func generateJiraDOT(g *jira.DependencyGraph, issueMap map[string]map[string]interface{}) string {
	var sb strings.Builder
	sb.WriteString("digraph G {\n")
	sb.WriteString("  rankdir=LR;\n")

	// Nodes
	for key := range g.AllTickets {
		issue, exists := issueMap[key]
		var summary string
		if exists {
			fields, _ := issue["fields"].(map[string]interface{})
			summary, _ = fields["summary"].(string)
		} else {
			summary = "External"
		}
		safeSummary := strings.ReplaceAll(summary, "\"", "'")
		if len(safeSummary) > 30 {
			safeSummary = safeSummary[:27] + "..."
		}

		sb.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\\n%s\"];\n", key, key, safeSummary))
	}

	// Edges
	for blocker, blockedList := range g.Blocks {
		for _, blocked := range blockedList {
			sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\";\n", blocker, blocked))
		}
	}

	sb.WriteString("}\n")
	return sb.String()
}
