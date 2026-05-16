package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

func buildJobInteractive(host string, wait bool) {
	scanner := bufio.NewScanner(stdin)

	prompt := func(msg string, required bool) string {
		for {
			fmt.Fprintf(stdout, "%s", msg)
			if !scanner.Scan() {
				return ""
			}
			text := strings.TrimSpace(scanner.Text())
			if required && text == "" {
				fmt.Fprintln(stdout, "This field is required. Please enter a value.")
				continue
			}
			return text
		}
	}

	fmt.Fprintln(stdout, "--- Interactive Job Builder ---")

	summary := prompt("Enter Summary (required): ", true)
	repoURL := prompt("Enter Repository URL (required): ", true)
	description := prompt("Enter Description (optional): ", false)
	if description == "" {
		description = summary
	}

	dependsOnStr := prompt("Enter Dependencies (comma-separated job IDs, optional): ", false)
	var dependsOn []string
	if dependsOnStr != "" {
		for _, dep := range strings.Split(dependsOnStr, ",") {
			depTrim := strings.TrimSpace(dep)
			if depTrim != "" {
				dependsOn = append(dependsOn, depTrim)
			}
		}
	}

	tagsStr := prompt("Enter Tags (comma-separated, optional): ", false)
	var tags []string
	if tagsStr != "" {
		for _, tag := range strings.Split(tagsStr, ",") {
			tagTrim := strings.TrimSpace(tag)
			if tagTrim != "" {
				tags = append(tags, tagTrim)
			}
		}
	}

	concurrencyGroup := prompt("Enter Concurrency Group (optional): ", false)

	envVars := make(map[string]string)
	fmt.Fprintln(stdout, "Enter Environment Variables (KEY=VALUE). Leave blank to finish env vars:")
	for {
		envLine := prompt("Env Var (or blank to finish): ", false)
		if envLine == "" {
			break
		}
		parts := strings.SplitN(envLine, "=", 2)
		if len(parts) == 2 {
			envVars[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		} else {
			fmt.Fprintln(stdout, "Invalid format. Please use KEY=VALUE.")
		}
	}

	id := "adhoc-" + uuid.New().String()[:8]

	jobData := map[string]interface{}{
		"id":                 id,
		"summary":            summary,
		"description":        description,
		"repo_url":           repoURL,
		"depends_on":         dependsOn,
		"tags":               tags,
		"concurrency_group":  concurrencyGroup,
		"env_vars":           envVars,
		"cancel_in_progress": true, // Sensible default for ad-hoc built jobs, but can be customized later if needed
	}

	jsonData, err := json.MarshalIndent(jobData, "", "  ")
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal job data: %v\n", err)
		exitFunc(1)
		return
	}

	// Show summary
	fmt.Fprintln(stdout, "\n--- Job Summary ---")
	fmt.Fprintln(stdout, string(jsonData))

	confirm := prompt("Submit this job? [Y/n]: ", false)
	// ⚡ Bolt: Use strings.EqualFold to avoid allocating a new string with strings.ToLower
	if strings.EqualFold(confirm, "n") || strings.EqualFold(confirm, "no") {
		fmt.Fprintln(stdout, "Job submission cancelled.")
		return
	}

	// Write to temp file and submit
	tmpFile, err := os.CreateTemp("", "recac-build-job-*.json")
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create temp file: %v\n", err)
		exitFunc(1)
		return
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(jsonData); err != nil {
		fmt.Fprintf(stdout, "Failed to write job data to temp file: %v\n", err)
		exitFunc(1)
		return
	}
	tmpFile.Close()

	// Wait before submitting so user sees the message
	time.Sleep(100 * time.Millisecond)

	submitJob(host, tmpFile.Name(), wait)
}
