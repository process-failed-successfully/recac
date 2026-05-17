package main

import (
	"fmt"
	"io"
	"net/http"
)

func exportJunit(host, outputFile string) {
	url := fmt.Sprintf("%s/jobs/export?format=junit", host)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to export JUnit XML: status %s - %s\n", resp.Status, string(body))
		exitFunc(1)
		return
	}

	w, f, err := getExportWriter(outputFile)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create output file: %v\n", err)
		exitFunc(1)
		return
	}
	if f != nil {
		defer f.Close()
	}

	if _, err := io.Copy(w, resp.Body); err != nil {
		fmt.Fprintf(stdout, "Failed to write JUnit XML to output: %v\n", err)
		exitFunc(1)
		return
	}

	if outputFile != "-" {
		fmt.Fprintf(stdout, "JUnit XML successfully exported to %s\n", outputFile)
	}
}
