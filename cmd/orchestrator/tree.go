package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"recac/internal/orchestrator"

	"github.com/charmbracelet/lipgloss"
)

func printTree(host string) {
	url := fmt.Sprintf("%s/jobs?state=all", host)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stdout, "Failed to fetch jobs: status %s\n", resp.Status)
		exitFunc(1)
		return
	}

	var jobs []orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	if len(jobs) == 0 {
		fmt.Fprintln(stdout, "No jobs found.")
		return
	}

	// Create a map of jobs by ID
	jobMap := make(map[string]orchestrator.JobInfo)
	// Create a map of jobs by dependency
	childrenMap := make(map[string][]string)

	for _, job := range jobs {
		jobMap[job.ID] = job
		for _, dep := range job.WorkItem.DependsOn {
			childrenMap[dep] = append(childrenMap[dep], job.ID)
		}
	}

	// Find root jobs (jobs that don't depend on any other job in the current list)
	var rootJobs []string
	for _, job := range jobs {
		if len(job.WorkItem.DependsOn) == 0 {
			rootJobs = append(rootJobs, job.ID)
		} else {
			// Check if all dependencies are missing from the current list (e.g. purged from history)
			// If all are missing, consider it a root job for rendering purposes
			allDepsMissing := true
			for _, dep := range job.WorkItem.DependsOn {
				if _, exists := jobMap[dep]; exists {
					allDepsMissing = false
					break
				}
			}
			if allDepsMissing {
				rootJobs = append(rootJobs, job.ID)
			}
		}
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Job Dependency Tree (%d Jobs)", len(jobs))))
	fmt.Fprintln(stdout, "")

	// Render the tree
	for _, root := range rootJobs {
		renderNode(root, jobMap, childrenMap, "", true)
	}
}

func renderNode(nodeID string, jobMap map[string]orchestrator.JobInfo, childrenMap map[string][]string, prefix string, isLast bool) {
	job, exists := jobMap[nodeID]
	if !exists {
		return
	}

	// Choose tree characters
	branch := "├── "
	if isLast {
		branch = "└── "
	}

	// Format node string
	idStyle := lipgloss.NewStyle().Bold(true)

	statusColor := "252"
	switch job.Status {
	case "Completed":
		statusColor = "42" // Green
	case "Failed":
		statusColor = "196" // Red
	case "Pending":
		statusColor = "214" // Orange
	case "Spawning", "Running", "Active":
		statusColor = "39" // Blue
	}

	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor))
	summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	nodeStr := fmt.Sprintf("%s (%s) %s",
		idStyle.Render(job.ID),
		statusStyle.Render(job.Status),
		summaryStyle.Render(limitString(job.Summary, 40)),
	)

	// Print current node
	fmt.Fprintf(stdout, "%s%s%s\n", prefix, branch, nodeStr)

	// Prepare prefix for children
	childPrefix := prefix
	if isLast {
		childPrefix += "    "
	} else {
		childPrefix += "│   "
	}

	// Recursively render children
	children := childrenMap[nodeID]
	for i, child := range children {
		renderNode(child, jobMap, childrenMap, childPrefix, i == len(children)-1)
	}
}
