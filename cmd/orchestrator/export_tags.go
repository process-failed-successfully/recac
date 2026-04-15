package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
)

type TagPerformance struct {
	Tag             string  `json:"tag"`
	TotalJobs       int     `json:"total_jobs"`
	SuccessfulJobs  int     `json:"successful_jobs"`
	FailedJobs      int     `json:"failed_jobs"`
	SuccessRate     float64 `json:"success_rate"`
	AverageDuration float64 `json:"average_duration"`
	AverageCost     float64 `json:"average_cost"`
	TotalCost       float64 `json:"total_cost"`
	TotalTokens     float64 `json:"total_tokens"`
}

type TagStatsResponse struct {
	Tags []TagPerformance `json:"tags"`
}

func exportTags(host, outFile, format string, limit int) {
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

	var out io.Writer
	if outFile == "-" || outFile == "" {
		out = os.Stdout
	} else {
		f, err := os.Create(outFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		out = f
	}

	if format == "csv" {
		writer := csv.NewWriter(out)
		writer.Write([]string{"Tag", "TotalJobs", "SuccessfulJobs", "FailedJobs", "SuccessRate", "AverageDuration", "AverageCost", "TotalCost", "TotalTokens"})
		for _, t := range stats.Tags {
			writer.Write([]string{
				t.Tag,
				strconv.Itoa(t.TotalJobs),
				strconv.Itoa(t.SuccessfulJobs),
				strconv.Itoa(t.FailedJobs),
				fmt.Sprintf("%.2f", t.SuccessRate),
				fmt.Sprintf("%.2f", t.AverageDuration),
				fmt.Sprintf("%.4f", t.AverageCost),
				fmt.Sprintf("%.4f", t.TotalCost),
				fmt.Sprintf("%.0f", t.TotalTokens),
			})
		}
		writer.Flush()
	} else {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(stats); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing JSON: %v\n", err)
			os.Exit(1)
		}
	}
	if outFile != "-" && outFile != "" {
		fmt.Printf("Tag analysis exported to %s\n", outFile)
	}
}
