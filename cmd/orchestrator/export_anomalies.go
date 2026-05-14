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

func exportAnomalies(host string, outPath string, format string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/analyze/anomalies", host))
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
		fmt.Fprintf(stdout, "Failed to fetch anomalies analysis: status %s\n%s\n", resp.Status, body)
		exitFunc(1)
		return
	}

	var anomalies []orchestrator.AnomalyReport
	if err := json.NewDecoder(resp.Body).Decode(&anomalies); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
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

		writer.Write([]string{"Job ID", "Model", "Status", "Duration", "Duration Dev", "Cost", "Cost Dev"})
		for _, a := range anomalies {
			writer.Write([]string{
				a.JobID,
				a.Model,
				a.Status,
				a.Duration.String(),
				fmt.Sprintf("%.2f", a.DurationDev),
				fmt.Sprintf("%.4f", a.Cost),
				fmt.Sprintf("%.2f", a.CostDev),
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
		if err := encoder.Encode(anomalies); err != nil {
			fmt.Fprintf(stdout, "Failed to encode anomalies stats to JSON: %v\n", err)
			exitFunc(1)
			return
		}
	}

	if f != nil {
		fmt.Fprintf(stdout, "Successfully exported anomalies analysis to %s in %s format\n", outPath, format)
	}
}
