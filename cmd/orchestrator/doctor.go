package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"recac/internal/orchestrator"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var doctorCmd = &cobra.Command{
	Use:           "doctor",
	Short:         "Check the environment for potential issues",
	Long:          `Runs a series of checks to ensure that the Orchestrator environment is set up correctly.`,
	RunE:          runDoctor,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	// initConfig is called by Cobra via OnInitialize

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "Check\tStatus\tMessage")
	fmt.Fprintln(w, "-----\t------\t-------")

	ctx := context.Background()
	hasError := false

	// Helper to print result
	printResult := func(name string, err error) {
		status := "PASS"
		msg := "OK"
		if err != nil {
			status = "FAIL"
			msg = err.Error()
			hasError = true
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", name, status, msg)
	}

	// 1. Config
	configFile := viper.ConfigFileUsed()
	if configFile != "" {
		printResult("Config File", nil)
	} else {
		// Not necessarily an error if env vars are used, but good to know
		fmt.Fprintf(w, "Config File\tWARN\tNo config file found (using defaults/env)\n")
	}

	// 2. Poller
	pollerType := viper.GetString("orchestrator.poller")
	if pollerType == "" {
		pollerType = "jira" // Default
	}

	switch pollerType {
	case "jira":
		// Try to fetch credentials from known keys
		jUrl := viper.GetString("config.jiraUrl")
		if jUrl == "" {
			jUrl = os.Getenv("JIRA_URL")
		}

		jUser := viper.GetString("config.jiraUsername")
		if jUser == "" {
			jUser = os.Getenv("JIRA_USERNAME")
		}

		jToken := viper.GetString("secrets.jiraApiToken")
		if jToken == "" {
			jToken = os.Getenv("JIRA_API_TOKEN")
		}

		printResult("Jira Connectivity", orchestrator.CheckJira(ctx, jUrl, jUser, jToken))

	case "github":
		token := viper.GetString("orchestrator.github_token")
		owner := viper.GetString("orchestrator.github_owner")
		repo := viper.GetString("orchestrator.github_repo")
		printResult("GitHub Connectivity", orchestrator.CheckGitHub(ctx, token, owner, repo))

	case "file", "filesystem":
		file := viper.GetString("orchestrator.work_file")
		if _, err := os.Stat(file); err == nil {
			printResult("Work File", nil)
		} else {
			printResult("Work File", fmt.Errorf("file not found: %s", file))
		}
	case "file-dir":
		dir := viper.GetString("orchestrator.watch_dir")
		if _, err := os.Stat(dir); err == nil {
			printResult("Watch Dir", nil)
		} else {
			printResult("Watch Dir", fmt.Errorf("directory not found: %s", dir))
		}
	}

	// 3. Spawner
	mode := viper.GetString("orchestrator.mode")
	if mode == "" {
		mode = "local" // Default
	}

	switch mode {
	case "local", "docker":
		printResult("Docker Connectivity", orchestrator.CheckDocker(ctx))
	case "k8s", "kubernetes":
		ns := viper.GetString("orchestrator.namespace")
		printResult("K8s Connectivity", orchestrator.CheckK8s(ctx, ns))
	}

	// 4. AI Provider
	provider := viper.GetString("orchestrator.agent_provider")
	if provider == "" {
		provider = "openrouter" // Default
	}

	// Try to find API key based on provider
	var apiKey string
	switch provider {
	case "openrouter":
		apiKey = viper.GetString("secrets.openrouterApiKey")
		if apiKey == "" {
			apiKey = os.Getenv("OPENROUTER_API_KEY")
		}
	case "openai":
		apiKey = viper.GetString("secrets.openaiApiKey")
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
	case "anthropic":
		apiKey = viper.GetString("secrets.anthropicApiKey")
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
	case "gemini":
		apiKey = viper.GetString("secrets.geminiApiKey")
		if apiKey == "" {
			apiKey = os.Getenv("GEMINI_API_KEY")
		}
	}

	printResult(fmt.Sprintf("AI Provider (%s)", provider), orchestrator.CheckAIProvider(provider, apiKey))

	w.Flush()

	if hasError {
		return fmt.Errorf("one or more checks failed")
	}
	return nil
}
