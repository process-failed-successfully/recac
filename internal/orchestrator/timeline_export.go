package orchestrator

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ExportTimelineToMermaid converts a list of jobs into a Mermaid Gantt chart string.
func ExportTimelineToMermaid(jobs []JobInfo) string {
	if len(jobs) == 0 {
		return "gantt\n    title Job Execution Timeline\n"
	}

	// Sort jobs by start time, falling back to ID if start times are equal or zero
	sortedJobs := make([]JobInfo, len(jobs))
	copy(sortedJobs, jobs)
	sort.Slice(sortedJobs, func(i, j int) bool {
		t1 := sortedJobs[i].StartTime
		t2 := sortedJobs[j].StartTime
		if t1.Equal(t2) {
			return sortedJobs[i].ID < sortedJobs[j].ID
		}
		if t1.IsZero() {
			return false // Push zero times to the end
		}
		if t2.IsZero() {
			return true
		}
		return t1.Before(t2)
	})

	var sb strings.Builder
	sb.WriteString("gantt\n")
	sb.WriteString("    title Job Execution Timeline\n")
	sb.WriteString("    dateFormat YYYY-MM-DDTHH:mm:ssZ\n")
	sb.WriteString("    axisFormat %H:%M:%S\n\n")

	// Group jobs by status into sections to make the timeline easier to read
	sections := []string{"Active", "Completed", "Failed", "Canceled", "Skipped", "Pending"}
	grouped := make(map[string][]JobInfo)

	for _, job := range sortedJobs {
		status := job.Status
		if status == "Running" || status == "Spawning" || status == "Retrying" {
			status = "Active"
		}

		// Map generic status or use "Other"
		mappedStatus := "Other"
		for _, s := range sections {
			if strings.EqualFold(status, s) {
				mappedStatus = s
				break
			}
		}
		grouped[mappedStatus] = append(grouped[mappedStatus], job)
	}

	now := time.Now()

	for _, section := range append(sections, "Other") {
		if len(grouped[section]) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("    section %s\n", section))

		for _, job := range grouped[section] {
			start := job.StartTime
			if start.IsZero() {
				start = now // Fallback for jobs that haven't started (e.g. pending)
			}

			end := job.EndTime
			if end.IsZero() {
				end = now // Use current time for ongoing jobs
			}

			// Ensure end is strictly after start, otherwise Mermaid Gantt might fail to render
			if !end.After(start) {
				end = start.Add(1 * time.Second)
			}

			// Format: name : modifier, id, start_time, end_time
			// Modifiers: done (completed), active (running), crit (failed/critical)
			modifier := ""
			switch section {
			case "Completed":
				modifier = "done, "
			case "Active":
				modifier = "active, "
			case "Failed", "Canceled":
				modifier = "crit, "
			}

			// Escape colons in job ID/Summary as they break mermaid syntax
			safeName := strings.ReplaceAll(job.ID, ":", "-")
			safeID := strings.ReplaceAll(job.ID, ":", "-")

			line := fmt.Sprintf("    %s :%s%s, %s, %s\n",
				safeName,
				modifier,
				safeID,
				start.Format(time.RFC3339),
				end.Format(time.RFC3339))
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
