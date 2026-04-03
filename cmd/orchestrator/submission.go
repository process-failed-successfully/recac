package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"

	"recac/internal/orchestrator"

	"github.com/google/uuid"
)

var (
	exitFunc           = os.Exit
	stdout   io.Writer = os.Stdout
	stdin    io.Reader = os.Stdin
)

func submitJob(host, filePath string, wait bool) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to open file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}
	defer file.Close()

	// Verify JSON validity before sending (optional but good UX)
	var item map[string]interface{}
	if err := json.NewDecoder(file).Decode(&item); err != nil {
		fmt.Fprintf(stdout, "Invalid JSON in file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}
	// Extract ID for waiting
	id, _ := item["id"].(string)

	// Reset file pointer
	file.Seek(0, 0)

	resp, err := http.Post(fmt.Sprintf("%s/jobs", host), "application/json", file)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(stdout, "Failed to submit job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "%s\n", strings.TrimSpace(string(body)))

	if wait && id != "" {
		if err := waitForJob(host, id, stdout); err != nil {
			fmt.Fprintf(stdout, "Job failed: %v\n", err)
			exitFunc(1)
			return
		}
	}
}

func lintPipelineJob(filePath string, target string, vars map[string]string) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to read file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}

	baseDir := "."
	if filePath != "" {
		baseDir = filepath.Dir(filePath)
	}

	items, err := orchestrator.ParsePipelineToWorkItems(fileData, target, vars, baseDir)
	if err != nil {
		fmt.Fprintf(stdout, "Pipeline validation failed: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Pipeline is valid. Parsed %d jobs.\n", len(items))
}

func inspectPipelineJob(filePath string, target string, vars map[string]string) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to read file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}

	baseDir := "."
	if filePath != "" {
		baseDir = filepath.Dir(filePath)
	}

	items, err := orchestrator.ParsePipelineToWorkItems(fileData, target, vars, baseDir)
	if err != nil {
		fmt.Fprintf(stdout, "Pipeline inspection failed: %v\n", err)
		exitFunc(1)
		return
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		MarginBottom(1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Width(20)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Pipeline Inspection: %s (%d jobs)", filePath, len(items))))

	for i, item := range items {
		fmt.Fprintf(stdout, "%s %s\n", headerStyle.Render("Job ID:"), valueStyle.Render(item.ID))
		fmt.Fprintf(stdout, "%s %s\n", headerStyle.Render("Summary:"), valueStyle.Render(item.Summary))
		fmt.Fprintf(stdout, "%s %s\n", headerStyle.Render("Task:"), valueStyle.Render(limitString(item.Description, 100)))
		fmt.Fprintf(stdout, "%s %s\n", headerStyle.Render("Repo URL:"), valueStyle.Render(item.RepoURL))

		if len(item.DependsOn) > 0 {
			fmt.Fprintf(stdout, "%s %s\n", headerStyle.Render("Dependencies:"), valueStyle.Render(strings.Join(item.DependsOn, ", ")))
		}

		if len(item.Tags) > 0 {
			fmt.Fprintf(stdout, "%s %s\n", headerStyle.Render("Tags:"), valueStyle.Render(strings.Join(item.Tags, ", ")))
		}

		agentStr := fmt.Sprintf("%s / %s", item.AgentProvider, item.AgentModel)
		fmt.Fprintf(stdout, "%s %s\n", headerStyle.Render("Agent:"), valueStyle.Render(agentStr))

		if item.ConcurrencyGroup != "" {
			fmt.Fprintf(stdout, "%s %s\n", headerStyle.Render("Concurrency Group:"), valueStyle.Render(item.ConcurrencyGroup))
			fmt.Fprintf(stdout, "%s %v\n", headerStyle.Render("Cancel In Progress:"), valueStyle.Render(fmt.Sprintf("%v", item.CancelInProgress)))
		}

		if item.Timeout > 0 {
			fmt.Fprintf(stdout, "%s %s\n", headerStyle.Render("Timeout:"), valueStyle.Render(item.Timeout.String()))
		}
		if item.Delay > 0 {
			fmt.Fprintf(stdout, "%s %s\n", headerStyle.Render("Delay:"), valueStyle.Render(item.Delay.String()))
		}
		if item.RunCondition != "" {
			fmt.Fprintf(stdout, "%s %s\n", headerStyle.Render("Run Condition:"), valueStyle.Render(item.RunCondition))
		}

		if len(item.EnvVars) > 0 {
			fmt.Fprintln(stdout, headerStyle.Render("Environment Vars:"))
			for k, v := range item.EnvVars {
				fmt.Fprintf(stdout, "  %s=%s\n", k, valueStyle.Render(v))
			}
		}

		if i < len(items)-1 {
			fmt.Fprintln(stdout, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(strings.Repeat("-", 60)))
		}
	}
}

func importPipelineJob(host, filePath string, target string, vars map[string]string) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to read file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}

	var urlStr string
	if target != "" {
		urlStr = fmt.Sprintf("%s/jobs/pipeline/import?target=%s", host, url.QueryEscape(target))
	} else {
		urlStr = fmt.Sprintf("%s/jobs/pipeline/import", host)
	}

	if len(vars) > 0 {
		sep := "?"
		if target != "" {
			sep = "&"
		}
		for k, v := range vars {
			urlStr += fmt.Sprintf("%svar=%s=%s", sep, url.QueryEscape(k), url.QueryEscape(v))
			sep = "&"
		}
	}

	resp, err := http.Post(urlStr, "application/x-yaml", bytes.NewReader(fileData))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			fmt.Fprintf(stdout, "Pipeline successfully imported, but failed to parse response.\n")
		} else {
			if submitted, ok := result["submitted"].([]interface{}); ok {
				fmt.Fprintf(stdout, "Successfully imported %d jobs:\n", len(submitted))
				for _, id := range submitted {
					fmt.Fprintf(stdout, " - %v\n", id)
				}
			}
			if errors, ok := result["errors"].([]interface{}); ok && len(errors) > 0 {
				fmt.Fprintf(stdout, "\nFailed to import %d jobs:\n", len(errors))
				for _, errStr := range errors {
					fmt.Fprintf(stdout, " - %v\n", errStr)
				}
				exitFunc(1)
				return
			}
		}
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to import pipeline: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}
}

func submitPipelineInteractiveJob(host, filePath string, wait bool, dryRun bool, target string, vars map[string]string) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to read file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}

	var p orchestrator.Pipeline
	if err := yaml.Unmarshal(fileData, &p); err != nil {
		fmt.Fprintf(stdout, "Failed to unmarshal pipeline YAML: %v\n", err)
		exitFunc(1)
		return
	}

	if p.Variables == nil {
		p.Variables = make(map[string]string)
	}

	// Apply passed-in variables to the initial map for editing
	for k, v := range vars {
		p.Variables[k] = v
	}

	if len(p.Variables) == 0 {
		fmt.Fprintf(stdout, "No variables found in pipeline. Proceeding with submission.\n")
		submitPipelineJob(host, filePath, wait, dryRun, target, vars)
		return
	}

	varsJSON, err := json.MarshalIndent(p.Variables, "", "  ")
	if err != nil {
		fmt.Fprintf(stdout, "Failed to format variables JSON: %v\n", err)
		exitFunc(1)
		return
	}

	tmpFile, err := os.CreateTemp("", "recac-pipeline-vars-*.json")
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create temp file: %v\n", err)
		exitFunc(1)
		return
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(varsJSON); err != nil {
		fmt.Fprintf(stdout, "Failed to write to temp file: %v\n", err)
		exitFunc(1)
		return
	}
	tmpFile.Close()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi" // Default fallback
	}

	shellCmd := fmt.Sprintf("%s \"$1\"", editor)
	cmd := exec.Command("sh", "-c", shellCmd, "--", tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stdout, "Editor exited with error: %v\n", err)
		exitFunc(1)
		return
	}

	modifiedJSON, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		fmt.Fprintf(stdout, "Failed to read modified JSON: %v\n", err)
		exitFunc(1)
		return
	}

	var updatedVars map[string]string
	if err := json.Unmarshal(modifiedJSON, &updatedVars); err != nil {
		fmt.Fprintf(stdout, "Failed to parse modified JSON: %v\n", err)
		exitFunc(1)
		return
	}

	// Any newly defined interactive vars will override the CLI arguments in priority
	finalVars := make(map[string]string)
	for k, v := range vars {
		finalVars[k] = v
	}
	for k, v := range updatedVars {
		finalVars[k] = v
	}

	submitPipelineJob(host, filePath, wait, dryRun, target, finalVars)
}

func submitPipelineJob(host, filePath string, wait bool, dryRun bool, target string, vars map[string]string) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to open file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}
	defer file.Close()

	if dryRun {
		fileData, err := io.ReadAll(file)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to read file %s: %v\n", filePath, err)
			exitFunc(1)
			return
		}

		baseDir := "."
		if filePath != "" {
			baseDir = filepath.Dir(filePath)
		}

		items, err := orchestrator.ParsePipelineToWorkItems(fileData, target, vars, baseDir)
		if err != nil {
			fmt.Fprintf(stdout, "Pipeline validation failed: %v\n", err)
			exitFunc(1)
			return
		}

		fmt.Fprintf(stdout, "Pipeline valid. Dry run generated %d jobs:\n", len(items))
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(items); err != nil {
			fmt.Fprintf(stdout, "Failed to encode items to JSON: %v\n", err)
			exitFunc(1)
			return
		}
		return
	}

	urlObj, err := url.Parse(fmt.Sprintf("%s/jobs/pipeline", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := urlObj.Query()
	if target != "" {
		q.Set("target", target)
	}
	for k, v := range vars {
		q.Add("var", fmt.Sprintf("%s=%s", k, v))
	}
	urlObj.RawQuery = q.Encode()
	urlStr := urlObj.String()

	resp, err := http.Post(urlStr, "application/x-yaml", file)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(stdout, "Failed to submit pipeline job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Submitted []string `json:"submitted"`
		Errors    []string `json:"errors"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(stdout, "Failed to parse pipeline response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Pipeline submission completed.\n")
	if len(result.Submitted) > 0 {
		fmt.Fprintf(stdout, "Successfully submitted jobs: %s\n", strings.Join(result.Submitted, ", "))
	}
	if len(result.Errors) > 0 {
		fmt.Fprintf(stdout, "Errors:\n")
		for _, e := range result.Errors {
			fmt.Fprintf(stdout, "  - %s\n", e)
		}
		if len(result.Submitted) == 0 {
			exitFunc(1)
			return
		}
	}

	if wait && len(result.Submitted) > 0 {
		if err := waitForJobs(host, result.Submitted, stdout); err != nil {
			fmt.Fprintf(stdout, "Pipeline wait failed: %v\n", err)
			exitFunc(1)
			return
		}
	}
}

func submitMatrixJob(host, filePath string, wait bool) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to open file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}
	defer file.Close()

	resp, err := http.Post(fmt.Sprintf("%s/jobs/matrix", host), "application/json", file)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(stdout, "Failed to submit matrix job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Submitted []string `json:"submitted"`
		Errors    []string `json:"errors"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(stdout, "Failed to parse matrix response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Matrix submission completed.\n")
	if len(result.Submitted) > 0 {
		fmt.Fprintf(stdout, "Successfully submitted jobs: %s\n", strings.Join(result.Submitted, ", "))
	}
	if len(result.Errors) > 0 {
		fmt.Fprintf(stdout, "Errors:\n")
		for _, e := range result.Errors {
			fmt.Fprintf(stdout, "  - %s\n", e)
		}
		if len(result.Submitted) == 0 {
			exitFunc(1)
			return
		}
	}

	if wait && len(result.Submitted) > 0 {
		if err := waitForJobs(host, result.Submitted, stdout); err != nil {
			fmt.Fprintf(stdout, "Matrix wait failed: %v\n", err)
			exitFunc(1)
			return
		}
	}
}

func submitBatchJob(host, filePath string, wait bool) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to open file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}
	defer file.Close()

	resp, err := http.Post(fmt.Sprintf("%s/jobs/batch", host), "application/json", file)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(stdout, "Failed to submit batch job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Submitted []string `json:"submitted"`
		Errors    []string `json:"errors"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(stdout, "Failed to parse batch response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Batch submission completed.\n")
	if len(result.Submitted) > 0 {
		fmt.Fprintf(stdout, "Successfully submitted jobs: %s\n", strings.Join(result.Submitted, ", "))
	}
	if len(result.Errors) > 0 {
		fmt.Fprintf(stdout, "Errors:\n")
		for _, e := range result.Errors {
			fmt.Fprintf(stdout, "  - %s\n", e)
		}
		// If there were any errors, exit with non-zero
		if len(result.Submitted) == 0 {
			exitFunc(1)
			return
		}
	}

	if wait && len(result.Submitted) > 0 {
		if err := waitForJobs(host, result.Submitted, stdout); err != nil {
			fmt.Fprintf(stdout, "Batch wait failed: %v\n", err)
			exitFunc(1)
			return
		}
	}
}

func cloneBulkJobs(host, match, tag string, priority *int, wait bool, envVars map[string]string, dependsOn []string, remapDeps bool) {
	overrides := struct {
		EnvVars           map[string]string `json:"env_vars,omitempty"`
		Priority          *int              `json:"priority,omitempty"`
		DependsOn         []string          `json:"depends_on,omitempty"`
		RemapDependencies bool              `json:"remap_dependencies,omitempty"`
	}{
		EnvVars:           envVars,
		Priority:          priority,
		DependsOn:         dependsOn,
		RemapDependencies: remapDeps,
	}

	payload, err := json.Marshal(overrides)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal overrides: %v\n", err)
		exitFunc(1)
		return
	}

	u, err := url.Parse(fmt.Sprintf("%s/jobs/clone/bulk", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	if match != "" {
		q.Set("match", match)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	u.RawQuery = q.Encode()

	resp, err := http.Post(u.String(), "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(stdout, "Failed to clone jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Cloned       int      `json:"cloned"`
		ClonedJobIDs []string `json:"cloned_job_ids"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(stdout, "Failed to parse response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully cloned %d jobs.\n", result.Cloned)
	for _, id := range result.ClonedJobIDs {
		fmt.Fprintf(stdout, "- %s\n", id)
	}

	if wait && len(result.ClonedJobIDs) > 0 {
		if err := waitForJobs(host, result.ClonedJobIDs, stdout); err != nil {
			fmt.Fprintf(stdout, "Bulk clone wait failed: %v\n", err)
			exitFunc(1)
			return
		}
	}
}

func cloneJob(host, originalID, newID string, priority *int, wait bool, envVars map[string]string, dependsOn []string) {
	overrides := struct {
		NewID     string            `json:"new_id,omitempty"`
		EnvVars   map[string]string `json:"env_vars,omitempty"`
		Priority  *int              `json:"priority,omitempty"`
		DependsOn []string          `json:"depends_on,omitempty"`
	}{
		NewID:     newID,
		EnvVars:   envVars,
		Priority:  priority,
		DependsOn: dependsOn,
	}

	payload, err := json.Marshal(overrides)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal overrides: %v\n", err)
		exitFunc(1)
		return
	}

	cloneURL := fmt.Sprintf("%s/jobs/%s/clone", host, url.PathEscape(originalID))
	resp, err := http.Post(cloneURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(stdout, "Failed to clone job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		ClonedJobID string `json:"cloned_job_id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(stdout, "Failed to parse response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s cloned successfully as %s\n", originalID, result.ClonedJobID)

	if wait {
		if err := waitForJob(host, result.ClonedJobID, stdout); err != nil {
			fmt.Fprintf(stdout, "Job failed: %v\n", err)
			exitFunc(1)
			return
		}
	}
}

func submitMatrixInlineJob(host, repo, task, id string, priority int, delay, timeout time.Duration, maxRetries *int, requireApproval *bool, retryDelay *time.Duration, retryBackoff *float64, wait bool, envVars map[string]string, dependsOn []string, tags []string, concurrencyGroup string, cancelInProgress bool, agentProvider string, agentModel string, runCondition string, webhookURL string, autoHeal bool, matrix map[string][]string) {
	if id == "" {
		id = uuid.New().String()
	}

	baseItem := orchestrator.WorkItem{
		ID:                     id,
		Summary:                task,
		Description:            task,
		RepoURL:                repo,
		EnvVars:                envVars,
		Priority:               priority,
		DependsOn:              dependsOn,
		Tags:                   tags,
		Delay:                  delay,
		Timeout:                timeout,
		ConcurrencyGroup:       concurrencyGroup,
		CancelInProgress:       cancelInProgress,
		AgentProvider:          agentProvider,
		AgentModel:             agentModel,
		MaxRetries:             maxRetries,
		RequireApproval:        requireApproval,
		RetryDelay:             retryDelay,
		RetryBackoffMultiplier: retryBackoff,
		RunCondition:           runCondition,
		WebhookURL:             webhookURL,
		AutoHeal:               autoHeal,
	}

	reqBody := struct {
		BaseItem orchestrator.WorkItem `json:"base_item"`
		Matrix   map[string][]string   `json:"matrix"`
	}{
		BaseItem: baseItem,
		Matrix:   matrix,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal inline matrix data: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.Post(fmt.Sprintf("%s/jobs/matrix", host), "application/json", bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(stdout, "Failed to submit inline matrix job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Submitted []string `json:"submitted"`
		Errors    []string `json:"errors"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(stdout, "Failed to parse matrix response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Inline matrix submission completed.\n")
	if len(result.Submitted) > 0 {
		fmt.Fprintf(stdout, "Successfully submitted jobs: %s\n", strings.Join(result.Submitted, ", "))
	}
	if len(result.Errors) > 0 {
		fmt.Fprintf(stdout, "Errors:\n")
		for _, e := range result.Errors {
			fmt.Fprintf(stdout, "  - %s\n", e)
		}
		if len(result.Submitted) == 0 {
			exitFunc(1)
			return
		}
	}

	if wait && len(result.Submitted) > 0 {
		if err := waitForJobs(host, result.Submitted, stdout); err != nil {
			fmt.Fprintf(stdout, "Matrix wait failed: %v\n", err)
			exitFunc(1)
			return
		}
	}
}

func submitAdHocJob(host, repo, task, id string, priority int, delay, timeout time.Duration, maxRetries *int, requireApproval *bool, retryDelay *time.Duration, retryBackoff *float64, wait bool, envVars map[string]string, dependsOn []string, tags []string, concurrencyGroup string, cancelInProgress bool, agentProvider string, agentModel string, runCondition string, webhookURL string, autoHeal bool) {
	if id == "" {
		id = uuid.New().String()
	}

	item := orchestrator.WorkItem{
		ID:                     id,
		Summary:                task, // Using task description as summary for ad-hoc
		Description:            task,
		RepoURL:                repo,
		EnvVars:                envVars,
		Priority:               priority,
		DependsOn:              dependsOn,
		Tags:                   tags,
		Delay:                  delay,
		Timeout:                timeout,
		ConcurrencyGroup:       concurrencyGroup,
		CancelInProgress:       cancelInProgress,
		AgentProvider:          agentProvider,
		AgentModel:             agentModel,
		MaxRetries:             maxRetries,
		RequireApproval:        requireApproval,
		RetryDelay:             retryDelay,
		RetryBackoffMultiplier: retryBackoff,
		RunCondition:           runCondition,
		WebhookURL:             webhookURL,
		AutoHeal:               autoHeal,
	}
	payload, err := json.Marshal(item)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal work item: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.Post(fmt.Sprintf("%s/jobs", host), "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(stdout, "Failed to submit job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "%s\n", strings.TrimSpace(string(body)))

	if wait {
		if err := waitForJob(host, id, stdout); err != nil {
			fmt.Fprintf(stdout, "Job failed: %v\n", err)
			exitFunc(1)
			return
		}
	}
}

func waitForTag(host, tag string, out io.Writer) error {
	fmt.Fprintf(out, "Waiting for jobs with tag '%s' to complete...\n", tag)

	urlStr := fmt.Sprintf("%s/jobs?state=all&tag=%s", host, url.QueryEscape(tag))

	for {
		resp, err := http.Get(urlStr)
		if err != nil {
			time.Sleep(1 * time.Millisecond)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			time.Sleep(1 * time.Millisecond)
			continue
		}

		var jobs []orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
			resp.Body.Close()
			time.Sleep(1 * time.Millisecond)
			continue
		}
		resp.Body.Close()

		if len(jobs) == 0 {
			fmt.Fprintf(out, "No jobs found with tag '%s'.\n", tag)
			return nil
		}

		allCompleted := true
		for _, job := range jobs {
			if job.Status == "Failed" {
				return fmt.Errorf("job %s failed with error: %s", job.ID, job.Error)
			}
			if job.Status == "Canceled" {
				return fmt.Errorf("job %s canceled with error: %s", job.ID, job.Error)
			}
			if job.Status != "Completed" {
				allCompleted = false
			}
		}

		if allCompleted {
			fmt.Fprintf(out, "All jobs with tag '%s' completed successfully.\n", tag)
			return nil
		}

		time.Sleep(1 * time.Millisecond)
	}
}

func waitForJobs(host string, jobIDs []string, out io.Writer) error {
	fmt.Fprintf(out, "Waiting for %d jobs to complete...\n", len(jobIDs))

	remaining := make(map[string]bool)
	for _, id := range jobIDs {
		remaining[id] = true
	}

	for len(remaining) > 0 {
		for id := range remaining {
			resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, id))
			if err != nil {
				continue
			}

			var job orchestrator.JobInfo
			if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
				resp.Body.Close()
				continue
			}
			resp.Body.Close()

			if job.Status == "Completed" || job.Status == "Skipped" {
				fmt.Fprintf(out, "Job %s completed successfully.\n", id)
				delete(remaining, id)
			} else if job.Status == "Failed" {
				return fmt.Errorf("job %s failed with error: %s", id, job.Error)
			} else if job.Status == "Canceled" {
				return fmt.Errorf("job %s canceled with error: %s", id, job.Error)
			}
		}

		if len(remaining) > 0 {
			time.Sleep(1 * time.Millisecond)
		}
	}

	fmt.Fprintf(out, "All %d jobs completed successfully.\n", len(jobIDs))
	return nil
}

func waitForMatch(host, match string, out io.Writer) error {
	fmt.Fprintf(out, "Waiting for jobs matching '%s' to complete...\n", match)

	urlStr := fmt.Sprintf("%s/jobs?state=all&match=%s", host, url.QueryEscape(match))

	for {
		resp, err := http.Get(urlStr)
		if err != nil {
			time.Sleep(1 * time.Millisecond)
			continue
		}

		var jobs []orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
			resp.Body.Close()
			time.Sleep(1 * time.Millisecond)
			continue
		}
		resp.Body.Close()

		if len(jobs) == 0 {
			fmt.Fprintf(out, "No jobs found matching '%s'.\n", match)
			return nil
		}

		allCompleted := true
		for _, job := range jobs {
			if job.Status == "Failed" {
				return fmt.Errorf("job %s failed with error: %s", job.ID, job.Error)
			}
			if job.Status == "Canceled" {
				return fmt.Errorf("job %s canceled with error: %s", job.ID, job.Error)
			}
			if job.Status != "Completed" {
				allCompleted = false
			}
		}

		if allCompleted {
			fmt.Fprintf(out, "All jobs matching '%s' completed successfully.\n", match)
			return nil
		}

		time.Sleep(1 * time.Millisecond)
	}
}

func waitForJob(host, jobID string, out io.Writer) error {
	fmt.Fprintf(out, "Waiting for job %s to start...\n", jobID)

	// Poll until active or completed
	for {
		resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, jobID))
		if err != nil {
			// Retry on network error
			time.Sleep(1 * time.Millisecond)
			continue
		}

		var job orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
			resp.Body.Close()
			time.Sleep(1 * time.Millisecond)
			continue
		}
		resp.Body.Close()

		if job.Status == "Completed" {
			fmt.Fprintln(out, "Job already completed.")
			return nil
		}
		if job.Status == "Failed" {
			return fmt.Errorf("job failed with error: %s", job.Error)
		}
		if job.Status == "Canceled" {
			return fmt.Errorf("job canceled with error: %s", job.Error)
		}

		// If job is in a state where it might have logs (Running, Spawning which often implies running in Docker)
		// We try to stream logs.
		// Note: "Spawning" is the status during execution in DockerSpawner.
		if job.Status == "Spawning" || job.Status == "Running" || job.Status == "Active" {
			// Try to stream logs
			logsResp, err := http.Get(fmt.Sprintf("%s/jobs/%s/logs", host, jobID))
			if err == nil && logsResp.StatusCode == http.StatusOK {
				fmt.Fprintln(out, "--- Log Stream Start ---")
				io.Copy(out, logsResp.Body)
				logsResp.Body.Close()
				fmt.Fprintln(out, "\n--- Log Stream End ---")

				// Logs finished, check final status
				finalResp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, jobID))
				if err == nil {
					var finalJob orchestrator.JobInfo
					if err := json.NewDecoder(finalResp.Body).Decode(&finalJob); err == nil {
						if finalJob.Status == "Failed" {
							return fmt.Errorf("job failed with error: %s", finalJob.Error)
						}
						if finalJob.Status == "Canceled" {
							return fmt.Errorf("job canceled with error: %s", finalJob.Error)
						}
						// If logs finished, assume success unless failed?
						// But if status is still Spawning, it might be weird.
						// However, if streaming ends, container stopped.
						// Orchestrator moves to Completed/Failed AFTER container stops.
						// So there might be a tiny race where status is still Spawning.
						// We should probably loop again if status is still Spawning?
						if finalJob.Status == "Completed" {
							return nil
						}
					}
					finalResp.Body.Close()
				}
				// Continue loop to check status again
				continue
			} else {
				if logsResp != nil {
					logsResp.Body.Close()
				}
				// Maybe container not ready yet
				time.Sleep(1 * time.Millisecond)
				continue
			}
		}

		time.Sleep(1 * time.Millisecond)
	}
}

func clearPending(host string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/pending", host), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to clear pending jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	cleared, ok := result["cleared"].(float64)
	if !ok {
		fmt.Fprintf(stdout, "Unexpected response format\n")
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully cleared %d jobs from pending queue.\n", int(cleared))
}

func clearHistory(host string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/history", host), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to clear history: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	cleared, ok := result["cleared"].(float64)
	if !ok {
		fmt.Fprintf(stdout, "Unexpected response format\n")
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully cleared %d jobs from history.\n", int(cleared))
}

func cancelAllJobs(host string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/jobs", host), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to cancel jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	canceled, ok := result["canceled"].(float64)
	if !ok {
		fmt.Fprintf(stdout, "Unexpected response format\n")
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully canceled %d jobs.\n", int(canceled))
}

func cancelJobsByGroup(host, group string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("group", group)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to cancel jobs by concurrency group: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Canceled int `json:"canceled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully canceled %d jobs by concurrency group.\n", result.Canceled)
}

func cancelJobsByTag(host, tag string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/jobs?tag=%s", host, url.QueryEscape(tag)), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to cancel jobs by tag: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	canceled, ok := result["canceled"].(float64)
	if !ok {
		fmt.Fprintf(stdout, "Unexpected response format\n")
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully canceled %d jobs with tag '%s'.\n", int(canceled), tag)
}

func cancelJobsByStatus(host, status string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/jobs?status=%s", host, url.QueryEscape(status)), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to cancel jobs by status: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	canceled, ok := result["canceled"].(float64)
	if !ok {
		fmt.Fprintf(stdout, "Unexpected response format\n")
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully canceled %d jobs with status '%s'.\n", int(canceled), status)
}

func cancelJobsByMatch(host, match string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/jobs?match=%s", host, url.QueryEscape(match)), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to cancel jobs by match: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	canceled, ok := result["canceled"].(float64)
	if !ok {
		fmt.Fprintf(stdout, "Unexpected response format\n")
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully canceled %d jobs matching '%s'.\n", int(canceled), match)
}

func purgeJobsByTag(host, tag string) {
	urlStr := fmt.Sprintf("%s/history?tag=%s", host, url.QueryEscape(tag))
	req, err := http.NewRequest(http.MethodDelete, urlStr, nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to purge jobs by tag: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	cleared, ok := result["cleared"].(float64)
	if !ok {
		fmt.Fprintf(stdout, "Unexpected response format\n")
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully purged %d jobs with tag '%s'.\n", int(cleared), tag)
}

func purgeJobsByStatus(host, status string) {
	urlStr := fmt.Sprintf("%s/history?status=%s", host, url.QueryEscape(status))
	req, err := http.NewRequest(http.MethodDelete, urlStr, nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to purge jobs by status: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	cleared, ok := result["cleared"].(float64)
	if !ok {
		fmt.Fprintf(stdout, "Unexpected response format\n")
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully purged %d jobs with status '%s'.\n", int(cleared), status)
}

func purgeJobsByMatch(host, match string) {
	urlStr := fmt.Sprintf("%s/history?match=%s", host, url.QueryEscape(match))
	req, err := http.NewRequest(http.MethodDelete, urlStr, nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to purge jobs by match: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	cleared, ok := result["cleared"].(float64)
	if !ok {
		fmt.Fprintf(stdout, "Unexpected response format\n")
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully purged %d jobs matching '%s'.\n", int(cleared), match)
}


func purgeJobsOlderThan(host, olderThan string) {
	urlStr := fmt.Sprintf("%s/history?older_than=%s", host, url.QueryEscape(olderThan))
	req, err := http.NewRequest(http.MethodDelete, urlStr, nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to purge jobs older than %s: %s\n", olderThan, strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	cleared, ok := result["cleared"].(float64)
	if !ok {
		fmt.Fprintf(stdout, "Unexpected response format\n")
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully purged %d jobs older than '%s'.\n", int(cleared), olderThan)
}

func updateDependencies(host, jobID string, deps []string) {
	urlStr := fmt.Sprintf("%s/jobs/%s/dependencies", host, url.PathEscape(jobID))

	reqBody := struct {
		DependsOn []string `json:"depends_on"`
	}{
		DependsOn: deps,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal dependency data: %v\n", err)
		exitFunc(1)
		return
	}

	req, err := http.NewRequest(http.MethodPut, urlStr, bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to update dependencies: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s dependencies updated to: %s\n", jobID, strings.Join(deps, ", "))
}

func updateBulkDependencies(host, match, tag string, deps []string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/dependencies", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	if match != "" {
		q.Set("match", match)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	u.RawQuery = q.Encode()

	reqBody := struct {
		DependsOn []string `json:"depends_on"`
	}{
		DependsOn: deps,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal dependency data: %v\n", err)
		exitFunc(1)
		return
	}

	req, err := http.NewRequest(http.MethodPut, u.String(), bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to update dependencies: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Updated int `json:"updated"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully updated dependencies for %d jobs to: %s\n", result.Updated, strings.Join(deps, ", "))
}

func updateEnvVars(host, jobID string, envVars map[string]string) {
	urlStr := fmt.Sprintf("%s/jobs/%s/env", host, url.PathEscape(jobID))

	reqBody := struct {
		EnvVars map[string]string `json:"env_vars"`
	}{
		EnvVars: envVars,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal environment variable data: %v\n", err)
		exitFunc(1)
		return
	}

	req, err := http.NewRequest(http.MethodPut, urlStr, bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to update environment variables: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var pairs []string
	for k, v := range envVars {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	fmt.Fprintf(stdout, "Job %s environment variables updated to: %s\n", jobID, strings.Join(pairs, ", "))
}



func updateAgent(host, jobID, providerStr, modelStr string) {
	urlStr := fmt.Sprintf("%s/jobs/%s/agent", host, jobID)

	reqBody := struct {
		AgentProvider string `json:"agent_provider"`
		AgentModel    string `json:"agent_model"`
	}{
		AgentProvider: providerStr,
		AgentModel:    modelStr,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal agent data: %v\n", err)
		exitFunc(1)
		return
	}

	req, err := http.NewRequest(http.MethodPut, urlStr, bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to update agent: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s agent updated to provider=%s model=%s\n", jobID, providerStr, modelStr)
}

func updateBulkTimeout(host, match, tag, timeoutStr string) {
	reqBody := struct {
		Timeout string `json:"timeout"`
	}{
		Timeout: timeoutStr,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to encode request: %v\n", err)
		exitFunc(1)
		return
	}

	u, err := url.Parse(fmt.Sprintf("%s/jobs/timeout", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	if match != "" {
		q.Set("match", match)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodPut, u.String(), bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to update bulk timeouts: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Updated int `json:"updated"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully updated timeouts for %d pending jobs.\n", result.Updated)
}



func updateTimeout(host, jobID, timeoutStr string) {
	urlStr := fmt.Sprintf("%s/jobs/%s/timeout", host, jobID)
	reqBody := fmt.Sprintf(`{"timeout": "%s"}`, timeoutStr)

	req, err := http.NewRequest(http.MethodPut, urlStr, strings.NewReader(reqBody))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to update timeout: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s timeout updated to %s\n", jobID, timeoutStr)
}

func setJobOutput(host, jobID, key, val string) {
	reqBody := struct {
		Outputs map[string]string `json:"outputs"`
	}{
		Outputs: map[string]string{key: val},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal output data: %v\n", err)
		exitFunc(1)
		return
	}

	urlStr := fmt.Sprintf("%s/jobs/%s/output", host, url.PathEscape(jobID))
	req, err := http.NewRequest(http.MethodPost, urlStr, bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to set job output: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully set output %s=%s for job %s\n", key, val, jobID)
}

func getJobOutput(host, jobID, key string) {
	urlStr := fmt.Sprintf("%s/jobs/%s", host, url.PathEscape(jobID))
	resp, err := http.Get(urlStr)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to get job info: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var job orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		fmt.Fprintf(stdout, "Failed to parse job info: %v\n", err)
		exitFunc(1)
		return
	}

	val, ok := job.Outputs[key]
	if !ok {
		fmt.Fprintf(stdout, "Output key '%s' not found for job %s\n", key, jobID)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "%s\n", val)
}

func setJobProgress(host, jobID string, progress *int, msg *string) {
	reqBody := struct {
		Progress      *int    `json:"progress,omitempty"`
		StatusMessage *string `json:"status_message,omitempty"`
	}{
		Progress:      progress,
		StatusMessage: msg,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal progress data: %v\n", err)
		exitFunc(1)
		return
	}

	urlStr := fmt.Sprintf("%s/jobs/%s/progress", host, url.PathEscape(jobID))
	req, err := http.NewRequest(http.MethodPut, urlStr, bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to set job progress: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully updated progress for job %s\n", jobID)
}

func addJobMetrics(host, jobID, key string, val float64) {
	reqBody := struct {
		Metrics map[string]float64 `json:"metrics"`
	}{
		Metrics: map[string]float64{key: val},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal metric data: %v\n", err)
		exitFunc(1)
		return
	}

	urlStr := fmt.Sprintf("%s/jobs/%s/metrics", host, url.PathEscape(jobID))
	req, err := http.NewRequest(http.MethodPost, urlStr, bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to add job metrics: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully added metric %s=%.2f for job %s\n", key, val, jobID)
}

func exportJobs(host, path, format string) {
	if format != "json" && format != "csv" && format != "junit" {
		fmt.Fprintf(stdout, "Error: Invalid export format '%s'. Must be 'json', 'csv', or 'junit'.\n", format)
		exitFunc(1)
		return
	}

	url := fmt.Sprintf("%s/jobs/export?format=%s", host, format)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to export jobs: %s\n", strings.TrimSpace(string(body)))
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
		fmt.Fprintf(stdout, "Jobs successfully exported to %s\n", path)
	}
}

func editJobInteractive(host, jobID string, requirePending bool) (*orchestrator.WorkItem, []byte, error) {
	// 1. Fetch current job
	resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, jobID))
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to connect to orchestrator at %s: %w", host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("Failed to fetch job details: %s", strings.TrimSpace(string(body)))
	}

	var job orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, nil, fmt.Errorf("Failed to decode job response: %w", err)
	}

	if requirePending && job.Status != "Pending" && job.Status != "Pending Approval" {
		return nil, nil, fmt.Errorf("Cannot edit job %s. It is currently %s.", jobID, job.Status)
	}

	// 2. Format WorkItem as JSON
	jobJSON, err := json.MarshalIndent(job.WorkItem, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to format job JSON: %w", err)
	}

	// 3. Create temp file
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("recac-job-%s-*.json", jobID))
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name()) // Clean up after we're done

	if _, err := tmpFile.Write(jobJSON); err != nil {
		return nil, nil, fmt.Errorf("Failed to write to temp file: %w", err)
	}
	tmpFile.Close()

	// 4. Open editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi" // Default fallback
	}

	// Wrap editor command in sh -c to properly support arguments in $EDITOR (e.g. "code --wait")
	// Pass the filename as an argument to avoid command injection via string interpolation
	shellCmd := fmt.Sprintf("%s \"$1\"", editor)
	cmd := exec.Command("sh", "-c", shellCmd, "--", tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("Editor exited with error: %w", err)
	}

	// 5. Read back modified JSON
	modifiedJSON, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to read modified JSON: %w", err)
	}

	// 6. Validate
	var updatedItem orchestrator.WorkItem
	if err := json.Unmarshal(modifiedJSON, &updatedItem); err != nil {
		return nil, nil, fmt.Errorf("Failed to parse modified JSON: %w", err)
	}

	return &updatedItem, modifiedJSON, nil
}

func retryEditJob(host, jobID string, wait bool) {
	updatedItem, _, err := editJobInteractive(host, jobID, false)
	if err != nil {
		fmt.Fprintf(stdout, "%v\n", err)
		exitFunc(1)
		return
	}

	if updatedItem.ID == jobID {
		updatedItem.ID = updatedItem.ID + "-retry"
	}

	payload, err := json.Marshal(updatedItem)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal modified job: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.Post(fmt.Sprintf("%s/jobs", host), "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(stdout, "Failed to submit retried job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s submitted successfully.\n", updatedItem.ID)

	if wait {
		if err := waitForJob(host, updatedItem.ID, stdout); err != nil {
			fmt.Fprintf(stdout, "Job failed: %v\n", err)
			exitFunc(1)
			return
		}
	}
}

func editJob(host, jobID string) {
	updatedItem, modifiedJSON, err := editJobInteractive(host, jobID, true)
	if err != nil {
		fmt.Fprintf(stdout, "%v\n", err)
		exitFunc(1)
		return
	}

	if updatedItem.ID != jobID {
		fmt.Fprintf(stdout, "Error: You cannot change the Job ID during edit.\n")
		exitFunc(1)
		return
	}

	// 7. Send PUT request
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/jobs/%s", host, jobID), bytes.NewBuffer(modifiedJSON))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create PUT request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	putResp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer putResp.Body.Close()

	if putResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(putResp.Body)
		fmt.Fprintf(stdout, "Failed to update job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s updated successfully.\n", jobID)
}

func updateBulkEnvVars(host, match, tag string, envVars map[string]string) {
	reqBody := struct {
		EnvVars map[string]string `json:"env_vars"`
	}{
		EnvVars: envVars,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to encode request: %v\n", err)
		exitFunc(1)
		return
	}

	u, err := url.Parse(fmt.Sprintf("%s/jobs/env", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	if match != "" {
		q.Set("match", match)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodPut, u.String(), bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to update bulk env vars: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Updated int `json:"updated"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully updated environment variables for %d pending jobs.\n", result.Updated)
}

func holdJobs(host, match, tag string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/hold", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	if match != "" {
		q.Set("match", match)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodPost, u.String(), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to hold jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Held int `json:"held"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully held %d jobs.\n", result.Held)
}

func unholdJobs(host, match, tag string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/unhold", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	if match != "" {
		q.Set("match", match)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodPost, u.String(), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to unhold jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Unheld int `json:"unheld"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully unheld %d jobs.\n", result.Unheld)
}

func renameJob(host, jobID, newID string) {
	reqBody := fmt.Sprintf(`{"new_id": "%s"}`, newID)
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/jobs/%s/rename", host, jobID), strings.NewReader(reqBody))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to rename job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s renamed successfully to %s.\n", jobID, newID)
}

func skipJob(host, jobID string) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/jobs/%s/skip", host, jobID), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to skip job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s skipped successfully.\n", jobID)
}

func skipJobs(host, match, tag string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/skip", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	if match != "" {
		q.Set("match", match)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodPost, u.String(), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to skip jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully skipped %d jobs.\n", result["skipped"])
}

func importJobs(host, path string) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to open file %s: %v\n", path, err)
		exitFunc(1)
		return
	}
	defer file.Close()

	var jobs []orchestrator.JobInfo
	if err := json.NewDecoder(file).Decode(&jobs); err != nil {
		fmt.Fprintf(stdout, "Invalid JSON in file %s: %v\n", path, err)
		exitFunc(1)
		return
	}

	var items []orchestrator.WorkItem
	for _, job := range jobs {
		items = append(items, job.WorkItem)
	}

	if len(items) == 0 {
		fmt.Fprintf(stdout, "No jobs found in file %s\n", path)
		return
	}

	data, err := json.Marshal(items)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to encode items: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.Post(fmt.Sprintf("%s/jobs/batch", host), "application/json", bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to import jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	body, _ := io.ReadAll(resp.Body)
	fmt.Fprintf(stdout, "Successfully imported jobs:\n%s\n", strings.TrimSpace(string(body)))
}


func updateBulkAgent(host, match, tag, providerStr, modelStr string) {
	reqBody := struct {
		AgentProvider string `json:"agent_provider"`
		AgentModel    string `json:"agent_model"`
	}{
		AgentProvider: providerStr,
		AgentModel:    modelStr,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to encode request: %v\n", err)
		exitFunc(1)
		return
	}

	u, err := url.Parse(fmt.Sprintf("%s/jobs/agent", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	if match != "" {
		q.Set("match", match)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodPut, u.String(), bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to update bulk agents: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Updated int `json:"updated"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully updated agents for %d pending jobs.\n", result.Updated)
}
