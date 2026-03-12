package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func exportPipeline(host, path string) {
	url := fmt.Sprintf("%s/jobs/export/pipeline?name=exported-pipeline", host)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to export pipeline: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	if path == "-" {
		// Output to stdout
		io.Copy(stdout, resp.Body)
	} else {
		// Output to file
		file, err := os.Create(path)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to create output file %s: %v\n", path, err)
			exitFunc(1)
			return
		}
		defer file.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to write to file %s: %v\n", path, err)
			exitFunc(1)
			return
		}
		fmt.Fprintf(stdout, "Pipeline successfully exported to %s\n", path)
	}
}
