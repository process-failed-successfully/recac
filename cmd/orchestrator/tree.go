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

func printJobTree(host string, jobID string) {
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

	// Create a map of jobs by ID
	jobMap := make(map[string]orchestrator.JobInfo)
	// Create a map of jobs by dependency
	childrenMap := make(map[string][]string)

	var targetJob *orchestrator.JobInfo
	for _, job := range jobs {
		jobMap[job.ID] = job
		for _, dep := range job.WorkItem.DependsOn {
			childrenMap[dep] = append(childrenMap[dep], job.ID)
		}
		if job.ID == jobID {
			targetJob = &job
		}
	}

	if targetJob == nil {
		fmt.Fprintf(stdout, "Job %s not found.\n", jobID)
		exitFunc(1)
		return
	}

	// BFS/DFS to find all ancestors and descendants
	relevantJobs := make(map[string]bool)

	// 1. Ancestors (dependencies)
	queue := []string{jobID}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if relevantJobs[curr] {
			continue
		}
		relevantJobs[curr] = true

		if job, exists := jobMap[curr]; exists {
			queue = append(queue, job.WorkItem.DependsOn...)
		}
	}

	// 2. Descendants (dependents)
	queue = []string{jobID}
	visitedDescendants := make(map[string]bool)
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if visitedDescendants[curr] {
			continue
		}
		visitedDescendants[curr] = true
		relevantJobs[curr] = true

		if children, exists := childrenMap[curr]; exists {
			queue = append(queue, children...)
		}
	}

	// Build a filtered children map containing only relevant edges
	filteredChildrenMap := make(map[string][]string)
	for id := range relevantJobs {
		if children, exists := childrenMap[id]; exists {
			for _, child := range children {
				if relevantJobs[child] {
					filteredChildrenMap[id] = append(filteredChildrenMap[id], child)
				}
			}
		}
	}

	// Find root jobs within the relevant subset
	var rootJobs []string
	for id := range relevantJobs {
		if job, exists := jobMap[id]; exists {
			// A job is a root if it has no dependencies IN THE RELEVANT SUBSET
			isRoot := true
			for _, dep := range job.WorkItem.DependsOn {
				if relevantJobs[dep] {
					isRoot = false
					break
				}
			}
			if isRoot {
				rootJobs = append(rootJobs, id)
			}
		}
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Job Dependency Tree for %s (%d Jobs)", jobID, len(relevantJobs))))
	fmt.Fprintln(stdout, "")

	// Render the tree
	for _, root := range rootJobs {
		renderNode(root, jobMap, filteredChildrenMap, "", true)
	}
}
