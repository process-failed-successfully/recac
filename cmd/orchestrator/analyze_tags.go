package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/tabwriter"
	"time"
)

func analyzeTags(host string, limit int, format string) {
	url := fmt.Sprintf("%s/jobs/analyze/tags?limit=%d", host, limit)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Error fetching tag analysis: %v\n", err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Error from server: %s\n", string(body))
		exitFunc(1)
		return
	}

	var stats TagStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		fmt.Fprintf(stdout, "Error decoding response: %v\n", err)
		exitFunc(1)
		return
	}

	if format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(stats); err != nil {
			fmt.Fprintf(stdout, "Error writing JSON: %v\n", err)
			exitFunc(1)
			return
		}
		return
	}

	fmt.Fprintln(stdout, "TAG PERFORMANCE ANALYSIS")
	fmt.Fprintln(stdout, "========================")

	if len(stats.Tags) == 0 {
		fmt.Fprintln(stdout, "No tag data available.")
		return
	}

	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TAG\tJOBS\tSUCCESS\tFAILED\tRATE\tAVG DUR\tAVG COST\tTOTAL COST")
	for _, t := range stats.Tags {
		durStr := time.Duration(t.AverageDuration).Truncate(time.Second).String()
		if t.AverageDuration < float64(time.Second) {
			durStr = time.Duration(t.AverageDuration).Truncate(time.Millisecond).String()
		}

		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%.1f%%\t%s\t$%.4f\t$%.2f\n",
			t.Tag, t.TotalJobs, t.SuccessfulJobs, t.FailedJobs, t.SuccessRate*100,
			durStr, t.AverageCost, t.TotalCost)
	}
	w.Flush()
}
