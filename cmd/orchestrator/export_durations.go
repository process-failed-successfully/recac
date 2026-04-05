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

// We need a local struct to unmarshal since the API response structure is anonymous in the handler.
type durationStatsResponse struct {
	TotalJobs      int     `json:"total_jobs"`
	TotalDuration  float64 `json:"total_duration_ms"`
	MeanDuration   float64 `json:"mean_duration_ms"`
	MedianDuration float64 `json:"median_duration_ms"`
	MinDuration    float64 `json:"min_duration_ms"`
	MaxDuration    float64 `json:"max_duration_ms"`
	TagStats       []struct {
		Tag          string  `json:"tag"`
		Count        int     `json:"count"`
		MeanDuration float64 `json:"mean_duration_ms"`
	} `json:"tag_stats"`
	TopSlowest []orchestrator.JobInfo `json:"top_slowest"`
}

func exportDurations(host string, outPath string, format string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/analyze/durations", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("limit", "1000") // large enough to fetch top slowest for export
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
		fmt.Fprintf(stdout, "Failed to fetch durations analysis: status %s\n%s\n", resp.Status, body)
		exitFunc(1)
		return
	}

	var stats durationStatsResponse
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
			"Total Duration (ms)",
			"Mean Duration (ms)",
			"Median Duration (ms)",
			"Min Duration (ms)",
			"Max Duration (ms)",
		})

		writer.Write([]string{
			fmt.Sprintf("%d", stats.TotalJobs),
			fmt.Sprintf("%.2f", stats.TotalDuration),
			fmt.Sprintf("%.2f", stats.MeanDuration),
			fmt.Sprintf("%.2f", stats.MedianDuration),
			fmt.Sprintf("%.2f", stats.MinDuration),
			fmt.Sprintf("%.2f", stats.MaxDuration),
		})

        writer.Write([]string{})
        writer.Write([]string{"Tag", "Count", "Mean Duration (ms)"})
        for _, tagStat := range stats.TagStats {
            writer.Write([]string{
                tagStat.Tag,
                fmt.Sprintf("%d", tagStat.Count),
                fmt.Sprintf("%.2f", tagStat.MeanDuration),
            })
        }

        writer.Write([]string{})
        writer.Write([]string{"Top Slowest Job ID", "Duration (ms)"})
        for _, job := range stats.TopSlowest {
             writer.Write([]string{
                job.ID,
                fmt.Sprintf("%.2f", float64(job.EndTime.Sub(job.StartTime).Milliseconds())),
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
			fmt.Fprintf(stdout, "Failed to encode duration stats to JSON: %v\n", err)
			exitFunc(1)
			return
		}
	}

	if f != nil {
		fmt.Fprintf(stdout, "Successfully exported duration analysis to %s in %s format\n", outPath, format)
	}
}
