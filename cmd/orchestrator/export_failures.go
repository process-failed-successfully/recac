package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"recac/internal/orchestrator"
)

type FailureStat struct {
	Summary     string   `json:"summary"`
	Occurrences int      `json:"occurrences"`
	JobIDs      []string `json:"job_ids"`
}

type FailuresExportResponse struct {
	TotalFailures int           `json:"total_failures"`
	Failures      []FailureStat `json:"failures"`
}

func exportFailures(host string, outPath string, format string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("state", "all")
	q.Set("status", "Failed")
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
		fmt.Fprintf(stdout, "Failed to fetch failed jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var jobs []orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	// Group jobs by summary
	summaryMap := make(map[string][]string) // Summary -> []JobIDs
	for _, job := range jobs {
		summary := strings.TrimSpace(job.Summary)
		if summary == "" {
			summary = "<empty summary>"
		}
		summaryMap[summary] = append(summaryMap[summary], job.ID)
	}

	var stats []FailureStat
	for summary, ids := range summaryMap {
		stats = append(stats, FailureStat{
			Summary:     summary,
			Occurrences: len(ids),
			JobIDs:      ids,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Occurrences != stats[j].Occurrences {
			return stats[i].Occurrences > stats[j].Occurrences
		}
		return stats[i].Summary < stats[j].Summary
	})

	exportData := FailuresExportResponse{
		TotalFailures: len(jobs),
		Failures:      stats,
	}

	var out io.Writer
	var f *os.File
	var exportErr error

	out, f, exportErr = getExportWriter(outPath)
	if exportErr != nil {
		fmt.Fprintf(stdout, "%v\n", exportErr)
		exitFunc(1)
		return
	}
	if f != nil {
		defer f.Close()
	}

	if format == "csv" {
		writer := csv.NewWriter(out)

		writer.Write([]string{
			"Summary",
			"Occurrences",
			"Job IDs",
		})

		for _, stat := range exportData.Failures {
			writer.Write([]string{
				stat.Summary,
				fmt.Sprintf("%d", stat.Occurrences),
				strings.Join(stat.JobIDs, ", "),
			})
		}

		writer.Flush()

		if err := writer.Error(); err != nil {
			fmt.Fprintf(stdout, "Failed to write CSV: %v\n", err)
			exitFunc(1)
			return
		}
	} else {
		// Default to JSON
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(exportData); err != nil {
			fmt.Fprintf(stdout, "Failed to encode failure stats to JSON: %v\n", err)
			exitFunc(1)
			return
		}
	}

	if f != nil {
		fmt.Fprintf(stdout, "Successfully exported failures analysis to %s in %s format\n", outPath, format)
	}
}
