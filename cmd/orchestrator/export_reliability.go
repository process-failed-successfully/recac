package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"


)

// Define local structures matching API response.
type flakyJobStat struct {
	Summary      string  `json:"summary"`
	Occurrences  int     `json:"occurrences"`
	TotalRetries int     `json:"total_retries"`
	AvgRetries   float64 `json:"avg_retries"`
}

type failedJobStat struct {
	Summary     string `json:"summary"`
	Occurrences int    `json:"occurrences"`
}

type reliabilityStatsResponse struct {
	TotalJobs      int             `json:"total_jobs"`
	SuccessfulJobs int             `json:"successful_jobs"`
	FailedJobs     int             `json:"failed_jobs"`
	FlakyJobs      int             `json:"flaky_jobs"`
	TotalRetries   int             `json:"total_retries"`
	SuccessRate    float64         `json:"success_rate"`
	FailureRate    float64         `json:"failure_rate"`
	FlakinessRate  float64         `json:"flakiness_rate"`
	TopFlakyJobs   []flakyJobStat  `json:"top_flaky_jobs"`
	TopFailingJobs []failedJobStat `json:"top_failing_jobs"`
}

func exportReliability(host string, outPath string, format string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/analyze/reliability", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("limit", "1000") // large enough to fetch top ones for export
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
		fmt.Fprintf(stdout, "Failed to fetch reliability analysis: status %s\n%s\n", resp.Status, body)
		exitFunc(1)
		return
	}

	var stats reliabilityStatsResponse
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
			"Total Jobs",
			"Successful Jobs",
			"Failed Jobs",
			"Flaky Jobs",
			"Total Retries",
			"Success Rate (%)",
			"Failure Rate (%)",
			"Flakiness Rate (%)",
		})

		writer.Write([]string{
			fmt.Sprintf("%d", stats.TotalJobs),
			fmt.Sprintf("%d", stats.SuccessfulJobs),
			fmt.Sprintf("%d", stats.FailedJobs),
			fmt.Sprintf("%d", stats.FlakyJobs),
			fmt.Sprintf("%d", stats.TotalRetries),
			fmt.Sprintf("%.2f", stats.SuccessRate),
			fmt.Sprintf("%.2f", stats.FailureRate),
			fmt.Sprintf("%.2f", stats.FlakinessRate),
		})

        writer.Write([]string{})
        writer.Write([]string{"Top Flaky Jobs"})
        writer.Write([]string{"Summary", "Occurrences", "Total Retries", "Avg Retries"})
        for _, flaky := range stats.TopFlakyJobs {
            writer.Write([]string{
                flaky.Summary,
                fmt.Sprintf("%d", flaky.Occurrences),
                fmt.Sprintf("%d", flaky.TotalRetries),
                fmt.Sprintf("%.2f", flaky.AvgRetries),
            })
        }

        writer.Write([]string{})
        writer.Write([]string{"Top Failing Jobs"})
        writer.Write([]string{"Summary", "Occurrences"})
        for _, failing := range stats.TopFailingJobs {
             writer.Write([]string{
                failing.Summary,
                fmt.Sprintf("%d", failing.Occurrences),
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
			fmt.Fprintf(stdout, "Failed to encode reliability stats to JSON: %v\n", err)
			exitFunc(1)
			return
		}
	}

	if f != nil {
		fmt.Fprintf(stdout, "Successfully exported reliability analysis to %s in %s format\n", outPath, format)
	}
}
