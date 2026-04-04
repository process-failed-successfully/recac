package main

import (
	"encoding/json"
	"fmt"
	"os"

	"recac/internal/orchestrator"
)

func listPipelineVarsJob(pipelineFile string, format string) {
	data, err := os.ReadFile(pipelineFile)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to read pipeline file: %v\n", err)
		exitFunc(1)
		return
	}

	vars, err := orchestrator.ExtractPipelineVariables(data)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse pipeline variables: %v\n", err)
		exitFunc(1)
		return
	}

	if format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(vars); err != nil {
			fmt.Fprintf(stdout, "Failed to encode pipeline variables to JSON: %v\n", err)
			exitFunc(1)
		}
		return
	}

	if len(vars) == 0 {
		fmt.Fprintln(stdout, "No variables found in the pipeline.")
		return
	}

	fmt.Fprintf(stdout, "Pipeline Variables (%d):\n", len(vars))
	for _, v := range vars {
		fmt.Fprintf(stdout, "- %s\n", v)
	}
}
