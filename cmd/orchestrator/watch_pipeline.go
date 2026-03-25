package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func watchPipelineJob(ctx context.Context, host, filePath string, dryRun bool, target string, vars map[string]string, runID string, interval time.Duration) {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Watching Pipeline: %s", filePath)))
	fmt.Fprintf(stdout, "Interval: %v\n\n", interval)

	// Do initial apply
	lastModTime := time.Time{}
	info, err := os.Stat(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Error reading file %s: %v\n", filePath, err)
	} else {
		lastModTime = info.ModTime()
		err := applyPipelineJobInternal(host, filePath, dryRun, target, vars, runID)
		if err != nil {
			fmt.Fprintf(stdout, "Error applying pipeline: %v\n", err)
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			info, err := os.Stat(filePath)
			if err != nil {
				// Don't spam errors if the user is in the middle of saving/renaming
				continue
			}

			modTime := info.ModTime()
			if modTime.After(lastModTime) {
				lastModTime = modTime
				fmt.Fprint(stdout, "\n"+strings.Repeat("-", 40)+"\n")
				fmt.Fprintf(stdout, "[%s] Detected change in %s, re-applying pipeline...\n", time.Now().Format(time.RFC3339), filePath)

				err := applyPipelineJobInternal(host, filePath, dryRun, target, vars, runID)
				if err != nil {
					fmt.Fprintf(stdout, "Error applying pipeline: %v\n", err)
				}
			}
		}
	}
}
