package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"

	"recac/internal/orchestrator"
)

func comparePipelines(pipelineFilesStr string) {
	parts := strings.Split(pipelineFilesStr, ",")
	if len(parts) != 2 {
		fmt.Fprintf(stdout, "Error: --compare-pipelines expects exactly two pipeline YAML files separated by a comma (e.g., p1.yaml,p2.yaml)\n")
		exitFunc(1)
		return
	}

	file1 := strings.TrimSpace(parts[0])
	file2 := strings.TrimSpace(parts[1])

	if file1 == "" || file2 == "" {
		fmt.Fprintf(stdout, "Error: Pipeline file paths cannot be empty\n")
		exitFunc(1)
		return
	}

	p1, err := parsePipeline(file1)
	if err != nil {
		fmt.Fprintf(stdout, "Error parsing pipeline 1 (%s): %v\n", file1, err)
		exitFunc(1)
		return
	}

	p2, err := parsePipeline(file2)
	if err != nil {
		fmt.Fprintf(stdout, "Error parsing pipeline 2 (%s): %v\n", file2, err)
		exitFunc(1)
		return
	}

	renderPipelineComparison(p1, p2, file1, file2)
}

func parsePipeline(filePath string) (*orchestrator.Pipeline, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var p orchestrator.Pipeline
	if err := yaml.Unmarshal(fileData, &p); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pipeline YAML: %w", err)
	}

	return &p, nil
}

func renderPipelineComparison(p1, p2 *orchestrator.Pipeline, file1, file2 string) {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		MarginBottom(1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Width(25)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Width(30).
		PaddingRight(2)

	diffStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Width(30).
		PaddingRight(2)

	missingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Width(30).
		PaddingRight(2)

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Comparing Pipelines: %s vs %s", file1, file2)))

	printRow := func(label, v1, v2 string, missing bool) {
		s1 := valueStyle
		s2 := valueStyle
		if v1 != v2 {
			s1 = diffStyle
			s2 = diffStyle
		}
		if missing {
			if v1 == "<missing>" {
				s1 = missingStyle
			}
			if v2 == "<missing>" {
				s2 = missingStyle
			}
		}
		fmt.Fprintf(stdout, "%s %s | %s\n", headerStyle.Render(label+":"), s1.Render(limitString(v1, 28)), s2.Render(limitString(v2, 28)))
	}

	printRow("Name", p1.Name, p2.Name, false)

	fmt.Fprintln(stdout, "\n"+lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("--- Jobs ---"))

	allJobs := make(map[string]bool)
	for k := range p1.Jobs {
		allJobs[k] = true
	}
	for k := range p2.Jobs {
		allJobs[k] = true
	}

	if len(allJobs) == 0 {
		fmt.Fprintln(stdout, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("No jobs in either pipeline."))
		return
	}

	for jobName := range allJobs {
		job1, ok1 := p1.Jobs[jobName]
		job2, ok2 := p2.Jobs[jobName]

		if !ok1 {
			printRow("Job: "+jobName, "<missing>", "present", true)
			continue
		}
		if !ok2 {
			printRow("Job: "+jobName, "present", "<missing>", true)
			continue
		}

		// Job is in both, compare fields
		differencesFound := false
		var jobDiffs []string

		compareField := func(field, v1, v2 string) {
			if v1 != v2 {
				differencesFound = true
				jobDiffs = append(jobDiffs, fmt.Sprintf("  %s: %s | %s", headerStyle.Render(field), diffStyle.Render(limitString(v1, 28)), diffStyle.Render(limitString(v2, 28))))
			}
		}

		compareField("Summary", job1.Summary, job2.Summary)
		compareField("Task", job1.Task, job2.Task)
		compareField("Description", job1.Description, job2.Description)
		compareField("RepoURL", job1.RepoURL, job2.RepoURL)
		compareField("AgentProvider", job1.AgentProvider, job2.AgentProvider)
		compareField("AgentModel", job1.AgentModel, job2.AgentModel)
		compareField("Delay", job1.Delay, job2.Delay)
		compareField("Timeout", job1.Timeout, job2.Timeout)
		compareField("ConcurrencyGroup", job1.ConcurrencyGroup, job2.ConcurrencyGroup)
		compareField("RunCondition", job1.RunCondition, job2.RunCondition)

		continueOnError1 := "false"
		if job1.ContinueOnError != nil && *job1.ContinueOnError {
			continueOnError1 = "true"
		}
		continueOnError2 := "false"
		if job2.ContinueOnError != nil && *job2.ContinueOnError {
			continueOnError2 = "true"
		}
		compareField("ContinueOnError", continueOnError1, continueOnError2)
		cancel1 := "false"
		if job1.CancelInProgress != nil && *job1.CancelInProgress {
			cancel1 = "true"
		}
		cancel2 := "false"
		if job2.CancelInProgress != nil && *job2.CancelInProgress {
			cancel2 = "true"
		}
		compareField("CancelInProgress", cancel1, cancel2)

		maxRetries1 := "default"
		if job1.MaxRetries != nil {
			maxRetries1 = fmt.Sprintf("%d", *job1.MaxRetries)
		}
		maxRetries2 := "default"
		if job2.MaxRetries != nil {
			maxRetries2 = fmt.Sprintf("%d", *job2.MaxRetries)
		}
		compareField("MaxRetries", maxRetries1, maxRetries2)

		requireApproval1 := "default"
		if job1.RequireApproval != nil {
			requireApproval1 = fmt.Sprintf("%t", *job1.RequireApproval)
		}
		requireApproval2 := "default"
		if job2.RequireApproval != nil {
			requireApproval2 = fmt.Sprintf("%t", *job2.RequireApproval)
		}
		compareField("RequireApproval", requireApproval1, requireApproval2)

		// Compare EnvVars
		allEnvKeys := make(map[string]bool)
		for k := range job1.EnvVars {
			allEnvKeys[k] = true
		}
		for k := range job2.EnvVars {
			allEnvKeys[k] = true
		}
		for k := range allEnvKeys {
			v1, ok1 := job1.EnvVars[k]
			if !ok1 {
				v1 = "<missing>"
			}
			v2, ok2 := job2.EnvVars[k]
			if !ok2 {
				v2 = "<missing>"
			}
			if v1 != v2 {
				differencesFound = true
				s1 := diffStyle
				s2 := diffStyle
				if v1 == "<missing>" {
					s1 = missingStyle
				}
				if v2 == "<missing>" {
					s2 = missingStyle
				}
				jobDiffs = append(jobDiffs, fmt.Sprintf("  Env[%s]: %s | %s", k, s1.Render(limitString(v1, 28)), s2.Render(limitString(v2, 28))))
			}
		}

		// Compare DependsOn
		deps1 := strings.Join(job1.DependsOn, ", ")
		deps2 := strings.Join(job2.DependsOn, ", ")
		compareField("DependsOn", deps1, deps2)

		// Compare Tags
		tags1 := strings.Join(job1.Tags, ", ")
		tags2 := strings.Join(job2.Tags, ", ")
		compareField("Tags", tags1, tags2)

		if differencesFound {
			fmt.Fprintf(stdout, "%s:\n", headerStyle.Render("Job: "+jobName))
			for _, diff := range jobDiffs {
				fmt.Fprintln(stdout, diff)
			}
		} else {
			printRow("Job: "+jobName, "identical", "identical", false)
		}
	}
}
