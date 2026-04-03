package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"recac/internal/orchestrator"
)

func exportCosts(host string, outPath string, format string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/analyze/costs", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("limit", "0") // fetch all for export
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
		fmt.Fprintf(stdout, "Failed to fetch cost analysis: status %s\n%s\n", resp.Status, body)
		exitFunc(1)
		return
	}

	var stats orchestrator.CostStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	var out io.Writer
	var f *os.File

	if outPath == "-" || outPath == "" {
		out = stdout
	} else {
		f, err = os.Create(outPath)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to create output file: %v\n", err)
			exitFunc(1)
			return
		}
		defer f.Close()
		out = f
	}

	if format == "csv" {
		writer := csv.NewWriter(out)

		// Write Overall Stats
		writer.Write([]string{"Section", "Total Evaluated Jobs", "Total Cost", "Total Prompt Tokens", "Total Completion Tokens"})
		writer.Write([]string{
			"Overall",
			fmt.Sprintf("%d", stats.TotalStats.TotalJobs),
			fmt.Sprintf("%.4f", stats.TotalStats.TotalCost),
			fmt.Sprintf("%.0f", stats.TotalStats.TotalTokensPrompt),
			fmt.Sprintf("%.0f", stats.TotalStats.TotalTokensCompletion),
		})
		writer.Write([]string{}) // Empty row separator

		// Write Tag Stats
		if len(stats.TagStats) > 0 {
			writer.Write([]string{"Section", "Tag", "Jobs Count", "Total Cost"})
			for _, stat := range stats.TagStats {
				writer.Write([]string{
					"Tag Stats",
					stat.Tag,
					fmt.Sprintf("%d", stat.JobsCount),
					fmt.Sprintf("%.4f", stat.Cost),
				})
			}
			writer.Write([]string{}) // Empty row separator
		}

		// Write Model Stats
		if len(stats.ModelStats) > 0 {
			writer.Write([]string{"Section", "Model", "Jobs Count", "Total Cost"})
			for _, stat := range stats.ModelStats {
				writer.Write([]string{
					"Model Stats",
					stat.Model,
					fmt.Sprintf("%d", stat.JobsCount),
					fmt.Sprintf("%.4f", stat.Cost),
				})
			}
			writer.Write([]string{}) // Empty row separator
		}

		// Write Jobs
		if len(stats.TopExpensiveJobs) > 0 {
			writer.Write([]string{"Section", "Job ID", "Summary", "Cost", "Tokens"})
			for _, job := range stats.TopExpensiveJobs {
				cost := 0.0
				tokens := 0.0
				if c, ok := job.Metrics["cost_usd"]; ok {
					cost = c
				}
				if t, ok := job.Metrics["tokens_total"]; ok {
					tokens = t
				} else {
					var p, c float64
					if pr, ok := job.Metrics["tokens_prompt"]; ok {
						p = pr
					}
					if co, ok := job.Metrics["tokens_completion"]; ok {
						c = co
					}
					tokens = p + c
				}

				writer.Write([]string{
					"Job Details",
					job.ID,
					job.Summary,
					fmt.Sprintf("%.4f", cost),
					fmt.Sprintf("%.0f", tokens),
				})
			}
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
		if err := encoder.Encode(stats); err != nil {
			fmt.Fprintf(stdout, "Failed to encode cost stats to JSON: %v\n", err)
			exitFunc(1)
			return
		}
	}

	if f != nil {
		fmt.Fprintf(stdout, "Successfully exported cost analysis to %s in %s format\n", outPath, format)
	}
}
