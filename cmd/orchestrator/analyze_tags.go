package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"
	"time"
)

func analyzeTags(host string, limit int, format string) {
	url := fmt.Sprintf("%s/jobs/analyze/tags?limit=%d", host, limit)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching tag analysis: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Error from server: %s\n", string(body))
		os.Exit(1)
	}

	var stats TagStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding response: %v\n", err)
		os.Exit(1)
	}

	if format == "json" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(stats); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing JSON: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Println("TAG PERFORMANCE ANALYSIS")
	fmt.Println("========================")

	if len(stats.Tags) == 0 {
		fmt.Println("No tag data available.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
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
