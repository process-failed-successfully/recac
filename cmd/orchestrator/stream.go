package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type sseEvent struct {
	Event     string                 `json:"event"`
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

func streamEvents(ctx context.Context, host string) error {
	url := fmt.Sprintf("%s/events", host)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to orchestrator at %s: %w", host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to subscribe to events: status %s", resp.Status)
	}

	fmt.Fprintf(stdout, "Listening for orchestrator events at %s...\n", host)

	// Styles
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	idStyle := lipgloss.NewStyle().Bold(true)

	// Event Type Styles
	colors := map[string]string{
		"connected":                "42",  // Green
		"job_completed":            "42",  // Green
		"job_failed":               "196", // Red
		"job_canceled":             "208", // Orange
		"job_spawning":             "39",  // Blue
		"job_retrying":             "214", // Yellow-Orange
		"job_skipped":              "244", // Gray
		"job_pending_approval":     "226", // Yellow
		"job_approved":             "42",  // Green
		"job_held":                 "208", // Orange
		"job_unheld":               "42",  // Green
		"job_deleted":              "196", // Red
		"job_purged":               "196", // Red
		"job_renamed":              "39",  // Blue
		"job_dependencies_updated": "39",  // Blue
		"job_env_updated":          "39",  // Blue
		"job_tags_updated":         "39",  // Blue
		"job_agent_updated":        "39",  // Blue
		"job_timeout_updated":      "39",  // Blue
		"job_priority_updated":     "39",  // Blue
		"job_workitem_updated":     "39",  // Blue
		"job_progress_updated":     "39",  // Blue
		"job_pending_deps":         "214", // Yellow-Orange
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		var event sseEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			fmt.Fprintf(stdout, "Error parsing event data: %v\n", err)
			continue
		}

		color := "252" // Default Light Gray
		if c, ok := colors[event.Event]; ok {
			color = c
		}
		eventStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true)

		now := time.Now().Format("15:04:05")

		if event.Event == "connected" {
			fmt.Fprintf(stdout, "%s [%s]\n",
				timeStyle.Render(now),
				eventStyle.Render(event.Event),
			)
			continue
		}

		var jobID string
		if id, ok := event.Data["id"].(string); ok {
			jobID = id
		} else if oldID, ok := event.Data["old_id"].(string); ok {
			jobID = oldID // For job_renamed
		}

		var summary string
		if sum, ok := event.Data["summary"].(string); ok {
			summary = limitString(sum, 50)
		} else if wi, ok := event.Data["work_item"].(map[string]interface{}); ok {
			if sum, ok := wi["summary"].(string); ok {
				summary = limitString(sum, 50)
			}
		}

		var details []string
		if event.Event == "job_progress_updated" {
			if prog, ok := event.Data["progress"].(float64); ok {
				details = append(details, fmt.Sprintf("Progress: %d%%", int(prog)))
			}
			if msg, ok := event.Data["status_message"].(string); ok {
				details = append(details, msg)
			}
		} else if event.Event == "job_renamed" {
			if newID, ok := event.Data["new_id"].(string); ok {
				details = append(details, fmt.Sprintf("New ID: %s", newID))
			}
		} else if event.Event == "job_failed" {
			if errStr, ok := event.Data["error"].(string); ok {
				details = append(details, fmt.Sprintf("Error: %s", errStr))
			}
		} else if event.Event == "job_retrying" {
			if retryCnt, ok := event.Data["retry_count"].(float64); ok {
				details = append(details, fmt.Sprintf("Retry Count: %d", int(retryCnt)))
			}
		} else if event.Event == "job_completed" || event.Event == "job_skipped" {
			if stStr, ok1 := event.Data["start_time"].(string); ok1 {
				if etStr, ok2 := event.Data["end_time"].(string); ok2 {
					st, err1 := time.Parse(time.RFC3339Nano, stStr)
					et, err2 := time.Parse(time.RFC3339Nano, etStr)
					if err1 == nil && err2 == nil {
						duration := et.Sub(st).Round(time.Second)
						details = append(details, fmt.Sprintf("Duration: %v", duration))
					}
				}
			}
		}

		outLine := fmt.Sprintf("%s [%s]", timeStyle.Render(now), eventStyle.Render(event.Event))
		if jobID != "" {
			outLine += fmt.Sprintf(" Job: %s", idStyle.Render(jobID))
		}
		if summary != "" {
			outLine += fmt.Sprintf(" | %s", summary)
		}
		if len(details) > 0 {
			outLine += fmt.Sprintf(" | %s", strings.Join(details, ", "))
		}

		fmt.Fprintln(stdout, outLine)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading event stream: %w", err)
	}

	fmt.Fprintln(stdout, "Event stream closed.")
	return nil
}
