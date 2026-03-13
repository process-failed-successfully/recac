package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/spf13/viper"
	"recac/internal/agent"
	"recac/internal/orchestrator"
)

// Allow overriding the agent factory for testing
var newAgentFunc = agent.NewAgent

func explainJob(host, jobID string, provider string, model string) {
	fmt.Fprintf(stdout, "Fetching job details for %s...\n", jobID)

	// 1. Fetch Job Metadata
	resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, jobID))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to fetch job details: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var job orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	// 2. Fetch Job Logs
	fmt.Fprintf(stdout, "Fetching logs for %s...\n", jobID)
	logResp, err := http.Get(fmt.Sprintf("%s/jobs/%s/logs", host, jobID))
	var logsText string
	if err != nil {
		fmt.Fprintf(stdout, "Warning: Failed to fetch logs: %v\n", err)
	} else {
		defer logResp.Body.Close()
		if logResp.StatusCode == http.StatusOK {
			logBytes, _ := io.ReadAll(logResp.Body)
			logsText = string(logBytes)
		} else {
			fmt.Fprintf(stdout, "Warning: Failed to fetch logs, status %d\n", logResp.StatusCode)
		}
	}

	// Extract the last 1000 lines of logs to avoid context overflow
	logLines := strings.Split(logsText, "\n")
	if len(logLines) > 1000 {
		logLines = logLines[len(logLines)-1000:]
		logsText = "... [Logs Truncated] ...\n" + strings.Join(logLines, "\n")
	}

	// 3. Initialize AI Client
	apiKey := viper.GetString("api_key")
	if apiKey == "" {
		// Try to fallback
		apiKey = viper.GetString("secrets.api_key")
	}

	if provider == "" {
		provider = viper.GetString("orchestrator.agent_provider")
	}
	if model == "" {
		model = viper.GetString("orchestrator.agent_model")
	}

	fmt.Fprintf(stdout, "Initializing AI using %s/%s...\n", provider, model)

	aiClient, err := newAgentFunc(provider, apiKey, model, "", "")
	if err != nil {
		fmt.Fprintf(stdout, "Failed to initialize AI agent: %v\n", err)
		exitFunc(1)
		return
	}

	// 4. Construct Prompt
	prompt := fmt.Sprintf(`You are an expert software engineer and debugger analyzing a failed or problematic job in an autonomous coding orchestrator.

Job ID: %s
Summary: %s
Status: %s
Error: %s

Here are the last log lines from the job execution:
%s

Analyze why the job failed or had issues, explain the root cause clearly, and suggest concrete steps to fix it.`,
		job.ID, job.Summary, job.Status, job.Error, logsText)

	fmt.Fprintf(stdout, "Analyzing with AI...\n\n")

	// 5. Stream or send prompt
	explanation, err := aiClient.Send(context.Background(), prompt)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to get explanation from AI: %v\n", err)
		exitFunc(1)
		return
	}

	// 6. Render the output nicely
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		// Fallback to plain text
		fmt.Fprintln(stdout, explanation)
		return
	}

	out, err := r.Render(explanation)
	if err != nil {
		fmt.Fprintln(stdout, explanation)
	} else {
		fmt.Fprint(stdout, out)
	}
}
