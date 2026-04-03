package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"text/tabwriter"

	"recac/internal/orchestrator"
)

func searchJobsGlobally(host, query, tag, status, format string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/search", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("q", query)
	if tag != "" {
		q.Set("tag", tag)
	}
	if status != "" {
		q.Set("status", status)
	}
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
		fmt.Fprintf(stdout, "Failed to search jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var jobs []orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		fmt.Fprintf(stdout, "Failed to decode search results: %v\n", err)
		exitFunc(1)
		return
	}

	if len(jobs) == 0 {
		fmt.Fprintln(stdout, "No matching jobs found.")
		return
	}

	if format == "json" {
		out, _ := json.MarshalIndent(jobs, "", "  ")
		fmt.Fprintln(stdout, string(out))
		return
	}

	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tSUMMARY\tTAGS")
	for _, job := range jobs {
		summary := job.Summary
		if len(summary) > 50 {
			summary = summary[:47] + "..."
		}
		tags := strings.Join(job.WorkItem.Tags, ", ")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", job.ID, job.Status, summary, tags)
	}
	w.Flush()
}
