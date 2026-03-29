package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"recac/internal/orchestrator"
)

func analyzeDurations(host string, limit int) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("state", "completed")
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to fetch completed jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var jobs []orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	// Filter out jobs that don't have valid start and end times
	var validJobs []orchestrator.JobInfo
	for _, job := range jobs {
		if !job.StartTime.IsZero() && !job.EndTime.IsZero() {
			dur := job.EndTime.Sub(job.StartTime)
			if dur > 0 {
				validJobs = append(validJobs, job)
			}
		}
	}

	if len(validJobs) == 0 {
		fmt.Fprintln(stdout, "No valid completed jobs with duration found.")
		return
	}

	// Calculate overall statistics
	var totalDuration time.Duration
	var minDuration time.Duration = -1
	var maxDuration time.Duration
	var durations []time.Duration

	// Tag grouping
	tagDurations := make(map[string][]time.Duration)

	for _, job := range validJobs {
		dur := job.EndTime.Sub(job.StartTime)
		totalDuration += dur
		durations = append(durations, dur)

		if minDuration == -1 || dur < minDuration {
			minDuration = dur
		}
		if dur > maxDuration {
			maxDuration = dur
		}

		for _, tag := range job.WorkItem.Tags {
			tagDurations[tag] = append(tagDurations[tag], dur)
		}
	}

	meanDuration := totalDuration / time.Duration(len(validJobs))

	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	var medianDuration time.Duration
	mid := len(durations) / 2
	if len(durations)%2 == 0 {
		medianDuration = (durations[mid-1] + durations[mid]) / 2
	} else {
		medianDuration = durations[mid]
	}

	// Sort jobs for Top N Slowest
	sort.Slice(validJobs, func(i, j int) bool {
		return validJobs[i].EndTime.Sub(validJobs[i].StartTime) > validJobs[j].EndTime.Sub(validJobs[j].StartTime)
	})

	if limit < 0 {
		limit = 0
	}
	topSlowest := validJobs
	if len(topSlowest) > limit {
		topSlowest = topSlowest[:limit]
	}

	// Tag statistics
	type tagStat struct {
		tag          string
		count        int
		meanDuration time.Duration
	}

	var tagStats []tagStat
	for tag, tagDurs := range tagDurations {
		var tagTotal time.Duration
		for _, d := range tagDurs {
			tagTotal += d
		}
		tagStats = append(tagStats, tagStat{
			tag:          tag,
			count:        len(tagDurs),
			meanDuration: tagTotal / time.Duration(len(tagDurs)),
		})
	}

	// Sort tag stats by mean duration descending
	sort.Slice(tagStats, func(i, j int) bool {
		return tagStats[i].meanDuration > tagStats[j].meanDuration
	})

	// Rendering
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("252")).
		Padding(0, 1)

	rowStyle := lipgloss.NewStyle().
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Width(15)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Duration Analysis (%d valid jobs)", len(validJobs))))
	fmt.Fprintln(stdout, "")

	printField := func(label, value string) {
		fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render(label+":"), valueStyle.Render(value))
	}

	printField("Total", totalDuration.Round(time.Second).String())
	printField("Mean", meanDuration.Round(time.Millisecond).String())
	printField("Median", medianDuration.Round(time.Millisecond).String())
	printField("Min", minDuration.Round(time.Millisecond).String())
	printField("Max", maxDuration.Round(time.Millisecond).String())

	fmt.Fprintln(stdout, "")

	if len(tagStats) > 0 {
		fmt.Fprintln(stdout, titleStyle.Render("Average Duration by Tag"))
		fmt.Fprintln(stdout, "")
		fmt.Fprintf(stdout, "%-20s %-10s %-20s\n",
			headerStyle.Render("Tag"),
			headerStyle.Render("Count"),
			headerStyle.Render("Mean Duration"),
		)
		for _, ts := range tagStats {
			fmt.Fprintf(stdout, "%-20s %-10d %-20s\n",
				rowStyle.Render(limitString(ts.tag, 18)),
				ts.count,
				rowStyle.Render(ts.meanDuration.Round(time.Millisecond).String()),
			)
		}
		fmt.Fprintln(stdout, "")
	}

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Top %d Slowest Jobs", len(topSlowest))))
	fmt.Fprintln(stdout, "")

	fmt.Fprintf(stdout, "%-15s %-40s %-15s %-15s\n",
		headerStyle.Render("ID"),
		headerStyle.Render("Summary"),
		headerStyle.Render("Status"),
		headerStyle.Render("Duration"),
	)

	for _, job := range topSlowest {
		dur := job.EndTime.Sub(job.StartTime).Round(time.Millisecond).String()
		fmt.Fprintf(stdout, "%-15s %-40s %-15s %-15s\n",
			rowStyle.Render(limitString(job.ID, 13)),
			rowStyle.Render(limitString(job.Summary, 38)),
			rowStyle.Render(limitString(job.Status, 13)),
			rowStyle.Render(dur),
		)
	}
}
