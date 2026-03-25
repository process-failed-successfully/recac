package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"

	"recac/internal/orchestrator"
)

func listTemplatesJob(filePath string, vars map[string]string) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to read file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}

	// Substitute variables manually as Pipeline.Parse does
	yamlStr := string(fileData)
	if len(vars) > 0 {
		yamlStr = os.Expand(yamlStr, func(k string) string {
			if v, ok := vars[k]; ok {
				return v
			}
			return "${" + k + "}"
		})
		fileData = []byte(yamlStr)
	}

	var p orchestrator.Pipeline
	if err := yaml.Unmarshal(fileData, &p); err != nil {
		fmt.Fprintf(stdout, "Failed to unmarshal pipeline YAML: %v\n", err)
		exitFunc(1)
		return
	}

	if len(p.Templates) == 0 {
		fmt.Fprintf(stdout, "No templates defined in pipeline %s\n", filePath)
		return
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	templateStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Templates in Pipeline: %s", filePath)))
	fmt.Fprintln(stdout, "")

	// Extract and sort template keys
	var keys []string
	for k := range p.Templates {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		t := p.Templates[k]
		fmt.Fprintf(stdout, "%s\n", templateStyle.Render(fmt.Sprintf("Template: %s", k)))

		if t.Summary != "" {
			fmt.Fprintf(stdout, "  %s %s\n", labelStyle.Render("Summary:"), t.Summary)
		}
		if t.Description != "" {
			fmt.Fprintf(stdout, "  %s %s\n", labelStyle.Render("Description:"), limitString(t.Description, 100))
		}
		if t.Task != "" {
			fmt.Fprintf(stdout, "  %s %s\n", labelStyle.Render("Task:"), limitString(t.Task, 100))
		}
		if t.Extends != "" {
			fmt.Fprintf(stdout, "  %s %s\n", labelStyle.Render("Extends:"), t.Extends)
		}
		if t.Stage != "" {
			fmt.Fprintf(stdout, "  %s %s\n", labelStyle.Render("Stage:"), t.Stage)
		}
		if t.RepoURL != "" {
			fmt.Fprintf(stdout, "  %s %s\n", labelStyle.Render("Repo URL:"), t.RepoURL)
		}
		if t.RunCondition != "" {
			fmt.Fprintf(stdout, "  %s %s\n", labelStyle.Render("Run Condition:"), t.RunCondition)
		}
		if len(t.DependsOn) > 0 {
			fmt.Fprintf(stdout, "  %s %s\n", labelStyle.Render("Depends On:"), strings.Join(t.DependsOn, ", "))
		}
		if len(t.Tags) > 0 {
			fmt.Fprintf(stdout, "  %s %s\n", labelStyle.Render("Tags:"), strings.Join(t.Tags, ", "))
		}
		if t.Priority != 0 {
			fmt.Fprintf(stdout, "  %s %d\n", labelStyle.Render("Priority:"), t.Priority)
		}
		if t.Timeout != "" {
			fmt.Fprintf(stdout, "  %s %s\n", labelStyle.Render("Timeout:"), t.Timeout)
		}
		if t.Delay != "" {
			fmt.Fprintf(stdout, "  %s %s\n", labelStyle.Render("Delay:"), t.Delay)
		}
		if t.ConcurrencyGroup != "" {
			fmt.Fprintf(stdout, "  %s %s\n", labelStyle.Render("Concurrency Group:"), t.ConcurrencyGroup)
		}
		if t.CancelInProgress != nil {
			fmt.Fprintf(stdout, "  %s %v\n", labelStyle.Render("Cancel In Progress:"), *t.CancelInProgress)
		}
		if t.AgentProvider != "" {
			fmt.Fprintf(stdout, "  %s %s\n", labelStyle.Render("Agent Provider:"), t.AgentProvider)
		}
		if t.AgentModel != "" {
			fmt.Fprintf(stdout, "  %s %s\n", labelStyle.Render("Agent Model:"), t.AgentModel)
		}
		if t.MaxRetries != nil {
			fmt.Fprintf(stdout, "  %s %d\n", labelStyle.Render("Max Retries:"), *t.MaxRetries)
		}
		if t.RequireApproval != nil {
			fmt.Fprintf(stdout, "  %s %v\n", labelStyle.Render("Require Approval:"), *t.RequireApproval)
		}
		if t.RetryDelay != "" {
			fmt.Fprintf(stdout, "  %s %s\n", labelStyle.Render("Retry Delay:"), t.RetryDelay)
		}
		if t.RetryBackoffMultiplier != nil {
			fmt.Fprintf(stdout, "  %s %.2f\n", labelStyle.Render("Retry Backoff:"), *t.RetryBackoffMultiplier)
		}

		if len(t.EnvVars) > 0 {
			fmt.Fprintf(stdout, "  %s\n", labelStyle.Render("Env Vars:"))
			// sort keys
			var envKeys []string
			for ek := range t.EnvVars {
				envKeys = append(envKeys, ek)
			}
			sort.Strings(envKeys)
			for _, ek := range envKeys {
				fmt.Fprintf(stdout, "    %s=%s\n", ek, t.EnvVars[ek])
			}
		}

		if len(t.Variables) > 0 {
			fmt.Fprintf(stdout, "  %s\n", labelStyle.Render("Variables:"))
			// sort keys
			var varKeys []string
			for vk := range t.Variables {
				varKeys = append(varKeys, vk)
			}
			sort.Strings(varKeys)
			for _, vk := range varKeys {
				fmt.Fprintf(stdout, "    %s=%s\n", vk, t.Variables[vk])
			}
		}

		if len(t.Matrix) > 0 {
			fmt.Fprintf(stdout, "  %s\n", labelStyle.Render("Matrix:"))
			var matKeys []string
			for mk := range t.Matrix {
				matKeys = append(matKeys, mk)
			}
			sort.Strings(matKeys)
			for _, mk := range matKeys {
				fmt.Fprintf(stdout, "    %s: [%s]\n", mk, strings.Join(t.Matrix[mk], ", "))
			}
		}

		fmt.Fprintln(stdout, "")
	}
}
