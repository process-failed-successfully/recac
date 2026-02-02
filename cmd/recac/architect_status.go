package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"recac/internal/architecture"
	"recac/internal/cmdutils"
	"recac/internal/jira"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var architectStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check implementation status of the architecture",
	Long:  "Compares the defined architecture against Jira tickets to calculate implementation progress.",
	RunE:  runArchitectStatus,
}

func init() {
	architectCmd.AddCommand(architectStatusCmd)
	architectStatusCmd.Flags().String("arch", ".recac/architecture/architecture.yaml", "Path to architecture.yaml")
	architectStatusCmd.Flags().String("project", "", "Jira project key")
}

func runArchitectStatus(cmd *cobra.Command, args []string) error {
	archPath, _ := cmd.Flags().GetString("arch")
	projectKey, _ := cmd.Flags().GetString("project")

	// Load Architecture
	archData, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("failed to read architecture file %s: %w", archPath, err)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(archData, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture: %w", err)
	}

	// Init Jira Client
	ctx := context.Background()
	client, err := cmdutils.GetJiraClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize Jira client: %w", err)
	}

	if projectKey == "" {
		projectKey, err = client.GetFirstProjectKey(ctx)
		if err != nil {
			return fmt.Errorf("failed to determine project key: %w", err)
		}
	}

	return calculateAndPrintStatus(ctx, client, &arch, projectKey, cmd.OutOrStdout())
}

func calculateAndPrintStatus(ctx context.Context, client jira.ClientInterface, arch *architecture.SystemArchitecture, projectKey string, out io.Writer) error {
	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "COMPONENT\tTYPE\tJIRA KEY\tSTATUS\tPROGRESS")

	totalComponents := len(arch.Components)
	completedComponents := 0

	for _, comp := range arch.Components {
		// Search for the component ticket
		// We look for ID:[COMPONENT_ID] in summary
		jql := fmt.Sprintf("project = \"%s\" AND summary ~ \"ID:[%s]*\"", projectKey, comp.ID)
		issues, err := client.SearchIssues(ctx, jql)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to search issues for %s: %v\n", comp.ID, err)
			continue
		}

		// Find the main ticket (usually the one where title starts with ID:[ID])
		var mainIssue map[string]interface{}
		for _, issue := range issues {
			fields, ok := issue["fields"].(map[string]interface{})
			if !ok {
				continue
			}
			summary, ok := fields["summary"].(string)
			if !ok {
				continue
			}
			// Loose matching to handle potential title changes, but ID:[ID] should be stable
			if strings.Contains(summary, fmt.Sprintf("ID:[%s]", comp.ID)) {
				mainIssue = issue
				break
			}
		}

		key := "N/A"
		status := "Missing"
		progress := "0%"

		if mainIssue != nil {
			key, _ = mainIssue["key"].(string)
			fields, _ := mainIssue["fields"].(map[string]interface{})
			if fields != nil {
				statusMap, _ := fields["status"].(map[string]interface{})
				if statusMap != nil {
					status, _ = statusMap["name"].(string)
				}
			}

			// Rough progress estimation based on status
			if isDone(status) {
				progress = "100%"
				completedComponents++
			} else if strings.EqualFold(status, "In Progress") {
				progress = "50%"
			} else if strings.EqualFold(status, "To Do") || strings.EqualFold(status, "Open") || strings.EqualFold(status, "Backlog") {
				progress = "0%"
			} else {
				progress = "0%" // Unknown status
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", comp.ID, comp.Type, key, status, progress)
	}

	fmt.Fprintln(w, "--------------------------------------------------------")
	overall := 0.0
	if totalComponents > 0 {
		overall = float64(completedComponents) / float64(totalComponents) * 100.0
	}
	fmt.Fprintf(w, "TOTAL\t%d Components\t\t\t%.1f%%\n", totalComponents, overall)

	return w.Flush()
}

func isDone(status string) bool {
	doneStatuses := []string{"Done", "Closed", "Resolved", "Finished", "Passed"}
	for _, s := range doneStatuses {
		if strings.EqualFold(s, status) {
			return true
		}
	}
	return false
}
