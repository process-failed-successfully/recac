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

func exportAgents(host string, outPath string, format string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/analyze/agents", host))
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
		fmt.Fprintf(stdout, "Failed to fetch agents analysis: status %s\n%s\n", resp.Status, body)
		exitFunc(1)
		return
	}

	var stats orchestrator.AgentStatsResponse
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

		writer.Write([]string{
			"Agent Provider",
			"Agent Model",
			"Total Jobs",
			"Successful Jobs",
			"Failed Jobs",
			"Success Rate",
			"Average Duration",
			"Average Cost",
			"Total Cost",
			"Total Tokens",
		})

		for _, stat := range stats.Agents {
			writer.Write([]string{
				stat.AgentProvider,
				stat.AgentModel,
				fmt.Sprintf("%d", stat.TotalJobs),
				fmt.Sprintf("%d", stat.SuccessfulJobs),
				fmt.Sprintf("%d", stat.FailedJobs),
				fmt.Sprintf("%.4f", stat.SuccessRate),
				stat.AverageDuration.String(),
				fmt.Sprintf("%.4f", stat.AverageCost),
				fmt.Sprintf("%.4f", stat.TotalCost),
				fmt.Sprintf("%.0f", stat.TotalTokens),
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
		if err := encoder.Encode(stats); err != nil {
			fmt.Fprintf(stdout, "Failed to encode agent stats to JSON: %v\n", err)
			exitFunc(1)
			return
		}
	}

	if f != nil {
		fmt.Fprintf(stdout, "Successfully exported agent analysis to %s in %s format\n", outPath, format)
	}
}
