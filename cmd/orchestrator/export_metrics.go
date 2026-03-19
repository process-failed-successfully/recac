package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func exportMetrics(host, outputFile, state string) {
	url := fmt.Sprintf("%s/jobs/export/metrics?state=%s", host, state)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to export metrics: status %s - %s\n", resp.Status, string(body))
		exitFunc(1)
		return
	}

	var out io.Writer = stdout
	if outputFile != "-" {
		f, err := os.Create(outputFile)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to create output file: %v\n", err)
			exitFunc(1)
			return
		}
		defer f.Close()
		out = f
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		fmt.Fprintf(stdout, "Failed to write metrics to output: %v\n", err)
		exitFunc(1)
		return
	}

	if outputFile != "-" {
		fmt.Fprintf(stdout, "Metrics successfully exported to %s\n", outputFile)
	}
}
