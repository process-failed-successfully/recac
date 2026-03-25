package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"

	"recac/internal/orchestrator"

	"github.com/charmbracelet/lipgloss"
)

func applyPipelineJob(host, filePath string, dryRun bool, target string, vars map[string]string, runID string) {
	err := applyPipelineJobInternal(host, filePath, dryRun, target, vars, runID)
	if err != nil {
		fmt.Fprintf(stdout, "%v\n", err)
		exitFunc(1)
	}
}

func applyPipelineJobInternal(host, filePath string, dryRun bool, target string, vars map[string]string, runID string) error {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("Failed to read file %s: %v", filePath, err)
	}

	baseDir := "."
	if filePath != "" {
		baseDir = filepath.Dir(filePath)
	}

	// Parse pipeline to generate work items with the specified runID
	// If runID is "stable", it skips suffix generation.
	items, err := orchestrator.ParsePipelineToWorkItemsWithRunID(fileData, target, vars, runID, baseDir)
	if err != nil {
		return fmt.Errorf("Pipeline validation failed: %v", err)
	}

	if len(items) == 0 {
		fmt.Fprintln(stdout, "No jobs generated from the pipeline.")
		return nil
	}

	// Fetch currently active jobs
	activeJobsResp, err := http.Get(fmt.Sprintf("%s/jobs?state=active", host))
	if err != nil {
		return fmt.Errorf("Failed to fetch active jobs: %v", err)
	}
	defer activeJobsResp.Body.Close()

	var activeJobs []orchestrator.JobInfo
	if activeJobsResp.StatusCode == http.StatusOK {
		json.NewDecoder(activeJobsResp.Body).Decode(&activeJobs)
	}

	activeJobMap := make(map[string]bool)
	for _, j := range activeJobs {
		activeJobMap[j.ID] = true
	}

	// Fetch currently pending jobs
	pendingJobsResp, err := http.Get(fmt.Sprintf("%s/jobs?state=pending", host))
	if err != nil {
		return fmt.Errorf("Failed to fetch pending jobs: %v", err)
	}
	defer pendingJobsResp.Body.Close()

	var pendingJobs []orchestrator.JobInfo
	if pendingJobsResp.StatusCode == http.StatusOK {
		json.NewDecoder(pendingJobsResp.Body).Decode(&pendingJobs)
	}

	pendingJobMap := make(map[string]orchestrator.WorkItem)
	for _, j := range pendingJobs {
		pendingJobMap[j.ID] = j.WorkItem
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	createStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true)
	updateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	skipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Bold(true)
	unchangedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

	modeLabel := "Applying Pipeline"
	if dryRun {
		modeLabel = "Dry Run Pipeline Apply"
	}
	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("%s: %s", modeLabel, filePath)))
	fmt.Fprintln(stdout, "")

	createdCount := 0
	updatedCount := 0
	skippedCount := 0
	unchangedCount := 0
	errorCount := 0

	for _, item := range items {
		if activeJobMap[item.ID] {
			fmt.Fprintf(stdout, "%s Job %s is currently active and cannot be updated\n", skipStyle.Render("[SKIP]"), item.ID)
			skippedCount++
			continue
		}

		if existingItem, exists := pendingJobMap[item.ID]; exists {
			// Compare fields (by comparing their marshaled JSON representations for deep equality)
			itemJSON, _ := json.Marshal(item)
			existingItemJSON, _ := json.Marshal(existingItem)

			if reflect.DeepEqual(itemJSON, existingItemJSON) {
				fmt.Fprintf(stdout, "%s Job %s\n", unchangedStyle.Render("[UNCHANGED]"), item.ID)
				unchangedCount++
				continue
			}

			// Fields changed, need to update
			if dryRun {
				fmt.Fprintf(stdout, "%s Job %s\n", updateStyle.Render("[UPDATE]"), item.ID)
				updatedCount++
			} else {
				payload, _ := json.Marshal(item)
				req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/jobs/%s", host, item.ID), bytes.NewBuffer(payload))
				if err != nil {
					fmt.Fprintf(stdout, "Error forming update request for %s: %v\n", item.ID, err)
					errorCount++
					continue
				}
				req.Header.Set("Content-Type", "application/json")

				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					fmt.Fprintf(stdout, "Failed to update job %s: %v\n", item.ID, err)
					errorCount++
					continue
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					fmt.Fprintf(stdout, "%s Job %s\n", updateStyle.Render("[UPDATE]"), item.ID)
					updatedCount++
				} else {
					bodyBytes, _ := io.ReadAll(resp.Body)
					fmt.Fprintf(stdout, "Error updating job %s: %s\n", item.ID, string(bodyBytes))
					errorCount++
				}
			}
		} else {
			// Job doesn't exist in active or pending, create it
			if dryRun {
				fmt.Fprintf(stdout, "%s Job %s\n", createStyle.Render("[CREATE]"), item.ID)
				createdCount++
			} else {
				payload, _ := json.Marshal(item)
				req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/jobs", host), bytes.NewBuffer(payload))
				if err != nil {
					fmt.Fprintf(stdout, "Error forming create request for %s: %v\n", item.ID, err)
					errorCount++
					continue
				}
				req.Header.Set("Content-Type", "application/json")

				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					fmt.Fprintf(stdout, "Failed to create job %s: %v\n", item.ID, err)
					errorCount++
					continue
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK {
					fmt.Fprintf(stdout, "%s Job %s\n", createStyle.Render("[CREATE]"), item.ID)
					createdCount++
				} else {
					bodyBytes, _ := io.ReadAll(resp.Body)
					fmt.Fprintf(stdout, "Error creating job %s: %s\n", item.ID, string(bodyBytes))
					errorCount++
				}
			}
		}
	}

	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "Summary: %d created, %d updated, %d unchanged, %d skipped, %d errors\n",
		createdCount, updatedCount, unchangedCount, skippedCount, errorCount)

	if errorCount > 0 {
		return fmt.Errorf("Applied pipeline with %d errors", errorCount)
	}

	return nil
}
