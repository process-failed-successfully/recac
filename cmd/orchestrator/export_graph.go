package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func exportGraph(host, path, format string) {
	if format != "mermaid" && format != "dot" && format != "plantuml" {
		fmt.Fprintf(stdout, "Invalid format: %s. Must be 'mermaid', 'dot', or 'plantuml'.\n", format)
		exitFunc(1)
		return
	}

	url := fmt.Sprintf("%s/jobs/export/graph?format=%s", host, format)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to export graph: %s\n", strings.TrimSpace(string(body)))
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
		fmt.Fprintf(stdout, "Graph successfully exported to %s\n", path)
	}
}
