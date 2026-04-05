package main

import (
	"fmt"
	"os"
	"path/filepath"

	"recac/internal/orchestrator"
)

func validatePipeline(filePath string, target string, vars map[string]string) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to read file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	items, err := orchestrator.ParsePipelineToWorkItems(fileData, target, vars, filepath.Dir(absPath))
	if err != nil {
		fmt.Fprintf(stdout, "Pipeline validation failed for %s:\n%v\n", filePath, err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Pipeline %s is valid.\nParsed %d valid job(s).\n", filePath, len(items))
}
