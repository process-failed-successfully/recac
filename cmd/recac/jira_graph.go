package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"recac/internal/cmdutils"
	"recac/internal/jira"

	"github.com/spf13/cobra"
)

var jiraGraphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Visualize Jira ticket dependencies",
	Long:  `Generates a Mermaid or DOT graph of Jira ticket dependencies based on 'Blocks' links.`,
	Run:   runJiraGraphCmd,
}

func init() {
	// Assuming jiraCmd is defined in jira.go
	jiraCmd.AddCommand(jiraGraphCmd)

	jiraGraphCmd.Flags().String("project", "", "Jira project key (e.g. PROJ)")
	jiraGraphCmd.Flags().String("label", "", "Jira label to filter by")
	jiraGraphCmd.Flags().String("format", "mermaid", "Output format (mermaid, dot)")
	jiraGraphCmd.Flags().Bool("include-done", false, "Include dependencies that are already Done")
}

func runJiraGraphCmd(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	project, _ := cmd.Flags().GetString("project")
	label, _ := cmd.Flags().GetString("label")
	format, _ := cmd.Flags().GetString("format")
	includeDone, _ := cmd.Flags().GetBool("include-done")

	if project == "" && label == "" {
		fmt.Fprintf(os.Stderr, "Error: You must specify either --project or --label\n")
		os.Exit(1)
	}

	client, err := cmdutils.GetJiraClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing Jira client: %v\n", err)
		os.Exit(1)
	}

	// 1. Construct JQL
	var conditions []string
	if project != "" {
		conditions = append(conditions, fmt.Sprintf("project = \"%s\"", project))
	}
	if label != "" {
		conditions = append(conditions, fmt.Sprintf("labels = \"%s\"", label))
	}
	jql := strings.Join(conditions, " AND ")

	fmt.Fprintf(os.Stderr, "Fetching issues with JQL: %s\n", jql)
	issues, err := client.SearchIssues(ctx, jql)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching issues: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Found %d issues.\n", len(issues))

	// 2. Build Graph
	// Map for quick lookup of issue details during generation
	issueMap := make(map[string]map[string]interface{})
	for _, issue := range issues {
		key, _ := issue["key"].(string)
		issueMap[key] = issue
	}

	getBlockers := func(issue map[string]interface{}) []string {
		var blockers []string
		fields, ok := issue["fields"].(map[string]interface{})
		if !ok {
			return nil
		}
		links, ok := fields["issuelinks"].([]interface{})
		if !ok {
			return nil
		}

		for _, link := range links {
			l, ok := link.(map[string]interface{})
			if !ok {
				continue
			}
			t, ok := l["type"].(map[string]interface{})
			if !ok {
				continue
			}
			inward, _ := t["inward"].(string)

			// Check for "is blocked by" relationship
			if strings.EqualFold(inward, "is blocked by") {
				inwardIssue, ok := l["inwardIssue"].(map[string]interface{})
				if !ok {
					continue
				}
				key, _ := inwardIssue["key"].(string)

				// Filter out Done items if requested
				if !includeDone {
					f, _ := inwardIssue["fields"].(map[string]interface{})
					if f != nil {
						s, _ := f["status"].(map[string]interface{})
						if s != nil {
							sn, _ := s["name"].(string)
							if isDoneStatus(sn) {
								continue
							}
						}
					}
				}
				blockers = append(blockers, key)
			}
		}
		return blockers
	}

	graph := jira.BuildGraphFromIssues(issues, getBlockers)

	// 3. Output
	switch strings.ToLower(format) {
	case "dot":
		fmt.Fprintln(cmd.OutOrStdout(), generateJiraDOT(graph, issueMap))
	default:
		fmt.Fprintln(cmd.OutOrStdout(), generateJiraMermaid(graph, issueMap))
	}
}

func generateJiraMermaid(g *jira.DependencyGraph, issueMap map[string]map[string]interface{}) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Helper to get status
	getStatus := func(key string) string {
		if issue, ok := issueMap[key]; ok {
			fields, _ := issue["fields"].(map[string]interface{})
			status, _ := fields["status"].(map[string]interface{})
			name, _ := status["name"].(string)
			return name
		}
		return "Unknown"
	}

	// Helper to get summary
	getSummary := func(key string) string {
		if issue, ok := issueMap[key]; ok {
			fields, _ := issue["fields"].(map[string]interface{})
			summary, _ := fields["summary"].(string)
			// Sanitize summary
			summary = strings.ReplaceAll(summary, "\"", "'")
			summary = strings.ReplaceAll(summary, "\n", " ")
			if len(summary) > 30 {
				summary = summary[:27] + "..."
			}
			return summary
		}
		return key
	}

	for key := range g.AllTickets {
		status := getStatus(key)
		summary := getSummary(key)

		style := ":::pending"
		if isDoneStatus(status) {
			style = ":::done"
		} else if strings.EqualFold(status, "In Progress") {
			style = ":::inprogress"
		}

		safeKey := strings.ReplaceAll(key, "-", "_")
		sb.WriteString(fmt.Sprintf("    %s[\"%s<br/>%s\"]%s\n", safeKey, key, summary, style))
	}

	// Edges
	// Iterate over BlockedBy to draw arrows from Blocker to Blocked
	for blocked, blockers := range g.BlockedBy {
		safeBlocked := strings.ReplaceAll(blocked, "-", "_")
		for _, blocker := range blockers {
			safeBlocker := strings.ReplaceAll(blocker, "-", "_")
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeBlocker, safeBlocked))
		}
	}

	// Styles
	sb.WriteString("\n    classDef done fill:#90EE90,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef inprogress fill:#87CEEB,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef pending fill:#FFFFFF,stroke:#333,stroke-width:1px,color:black;\n")

	return sb.String()
}

func generateJiraDOT(g *jira.DependencyGraph, issueMap map[string]map[string]interface{}) string {
	var sb strings.Builder
	sb.WriteString("digraph G {\n")
	sb.WriteString("  rankdir=TB;\n")
	sb.WriteString("  node [shape=box, style=filled];\n")

	for key := range g.AllTickets {
		safeKey := strings.ReplaceAll(key, "-", "_")

		color := "white"
		if issue, ok := issueMap[key]; ok {
			fields, _ := issue["fields"].(map[string]interface{})
			status, _ := fields["status"].(map[string]interface{})
			sn, _ := status["name"].(string)
			if isDoneStatus(sn) {
				color = "lightgreen"
			} else if strings.EqualFold(sn, "In Progress") {
				color = "lightblue"
			}
		}

		sb.WriteString(fmt.Sprintf("  %s [label=\"%s\", fillcolor=\"%s\"];\n", safeKey, key, color))
	}

	for blocked, blockers := range g.BlockedBy {
		safeBlocked := strings.ReplaceAll(blocked, "-", "_")
		for _, blocker := range blockers {
			safeBlocker := strings.ReplaceAll(blocker, "-", "_")
			sb.WriteString(fmt.Sprintf("  %s -> %s;\n", safeBlocker, safeBlocked))
		}
	}

	sb.WriteString("}\n")
	return sb.String()
}

// isDoneStatus is duplicated here or we can export it from client if needed.
// But client.go has it as private helper `isDoneStatus`.
// I'll redefine it locally to avoid dependency on private member.
func isDoneStatus(status string) bool {
	doneStatuses := []string{"Done", "Closed", "Resolved", "Finished", "Passed"}
	for _, s := range doneStatuses {
		if strings.EqualFold(s, status) {
			return true
		}
	}
	return false
}
