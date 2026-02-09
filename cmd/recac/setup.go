package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Wrapper for survey functions to allow mocking in tests
var (
	askOneFunc = survey.AskOne
)

// Wrapper for calling doctor command to allow mocking in tests
var runDoctorFunc = func(cmd *cobra.Command, args []string) {
	// Safely execute the doctor command logic
	if doctorCmd.Run != nil {
		doctorCmd.Run(cmd, args)
	} else if doctorCmd.RunE != nil {
		if err := doctorCmd.RunE(cmd, args); err != nil {
			fmt.Printf("Error running doctor: %v\n", err)
		}
	} else {
		fmt.Println("Error: doctor command has no Run or RunE defined")
	}
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactively set up RECAC configuration",
	Long:  `Runs an interactive wizard to configure RECAC settings, including provider, model, API keys, and Jira integration.`,
	RunE:  runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	fmt.Println("Welcome to RECAC Setup!")
	fmt.Println("-----------------------")

	answers := struct {
		Provider      string
		Model         string
		ApiKey        string
		SaveToEnv     bool
		JiraUrl       string
		JiraEmail     string
		JiraToken     string
		SaveJiraToEnv bool
		JiraLabel     string
		EnableSlack   bool
		SlackChannel  string
		SlackToken    string
	}{}

	// 1. Select Provider
	err := askOneFunc(&survey.Select{
		Message: "Choose your AI Provider:",
		Options: []string{"gemini", "openai", "anthropic", "openrouter", "ollama"},
		Default: "gemini",
	}, &answers.Provider)
	if err != nil {
		return err
	}

	// 2. Select Model (Default changes based on provider)
	defaultModel := "gemini-1.5-pro"
	switch answers.Provider {
	case "openai":
		defaultModel = "gpt-4-turbo"
	case "anthropic":
		defaultModel = "claude-3-opus"
	case "ollama":
		defaultModel = "llama3"
	}

	err = askOneFunc(&survey.Input{
		Message: "Enter the Model name:",
		Default: defaultModel,
	}, &answers.Model)
	if err != nil {
		return err
	}

	// 3. API Key
	err = askOneFunc(&survey.Password{
		Message: "Enter your API Key (leave empty to skip):",
	}, &answers.ApiKey)
	if err != nil {
		return err
	}

	if answers.ApiKey != "" {
		err = askOneFunc(&survey.Confirm{
			Message: "Do you want to save the API Key to a local .env file?",
			Default: true,
		}, &answers.SaveToEnv)
		if err != nil {
			return err
		}
	}

	// 4. Jira Configuration
	err = askOneFunc(&survey.Input{
		Message: "Enter your Jira URL (e.g., https://your-domain.atlassian.net):",
	}, &answers.JiraUrl, survey.WithValidator(func(ans interface{}) error {
		str, ok := ans.(string)
		if !ok || str == "" {
			return nil // Optional
		}
		u, err := url.ParseRequestURI(str)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("invalid URL")
		}
		return nil
	}))
	if err != nil {
		return err
	}

	if answers.JiraUrl != "" {
		err = askOneFunc(&survey.Input{
			Message: "Enter your Jira Email/Username:",
		}, &answers.JiraEmail)
		if err != nil {
			return err
		}

		err = askOneFunc(&survey.Password{
			Message: "Enter your Jira API Token:",
		}, &answers.JiraToken)
		if err != nil {
			return err
		}

		if answers.JiraToken != "" {
			err = askOneFunc(&survey.Confirm{
				Message: "Do you want to save the Jira Token to a local .env file?",
				Default: true,
			}, &answers.SaveJiraToEnv)
			if err != nil {
				return err
			}
		}

		err = askOneFunc(&survey.Input{
			Message: "Enter the Jira Label for agents to watch:",
			Default: "recac-agent",
		}, &answers.JiraLabel)
		if err != nil {
			return err
		}
	}

	// 5. Notifications
	err = askOneFunc(&survey.Confirm{
		Message: "Enable Slack notifications?",
		Default: false,
	}, &answers.EnableSlack)
	if err != nil {
		return err
	}

	if answers.EnableSlack {
		err = askOneFunc(&survey.Input{
			Message: "Slack Channel:",
			Default: "#general",
		}, &answers.SlackChannel)
		if err != nil {
			return err
		}
		err = askOneFunc(&survey.Password{
			Message: "Slack Bot Token:",
		}, &answers.SlackToken)
		if err != nil {
			return err
		}
	}

	// --- Saving Configuration ---

	// Update Viper settings
	viper.Set("provider", answers.Provider)
	viper.Set("model", answers.Model)

	if answers.JiraUrl != "" {
		viper.Set("jira.url", answers.JiraUrl)
		viper.Set("jira.username", answers.JiraEmail)
		viper.Set("orchestrator.jira_label", answers.JiraLabel)
		// If not saving to env, save to config (unencrypted) if desired?
		// For simplicity, if not env, we save to config to ensure functionality.
		if !answers.SaveJiraToEnv && answers.JiraToken != "" {
			viper.Set("jira.api_token", answers.JiraToken)
		}
	}

	if answers.EnableSlack {
		viper.Set("notifications.slack.enabled", true)
		viper.Set("notifications.slack.channel", answers.SlackChannel)
	}

	// Determine Config File Path
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			// Fallback to current directory if home fails
			configFile = "config.yaml"
		} else {
			configFile = filepath.Join(home, ".recac.yaml")
		}
	}

	if err := viper.WriteConfigAs(configFile); err != nil {
		fmt.Printf("Warning: Could not write %s: %v\n", configFile, err)
	} else {
		fmt.Printf("Configuration saved to %s\n", configFile)
	}

	// Write to .env
	var linesToAppend []string

	// Helper to append env var
	appendEnv := func(key, value string) {
		linesToAppend = append(linesToAppend, fmt.Sprintf("%s=%s", key, value))
	}

	if answers.SaveToEnv && answers.ApiKey != "" {
		envKey := ""
		switch answers.Provider {
		case "gemini":
			envKey = "GEMINI_API_KEY"
		case "openai":
			envKey = "OPENAI_API_KEY"
		case "anthropic":
			envKey = "ANTHROPIC_API_KEY"
		default:
			envKey = fmt.Sprintf("%s_API_KEY", strings.ToUpper(answers.Provider))
		}
		appendEnv(envKey, answers.ApiKey)
	}

	if answers.SaveJiraToEnv && answers.JiraToken != "" {
		appendEnv("JIRA_API_TOKEN", answers.JiraToken)
	}

	if answers.EnableSlack && answers.SlackToken != "" {
		appendEnv("SLACK_BOT_USER_TOKEN", answers.SlackToken)
	}

	if len(linesToAppend) > 0 {
		// Read existing .env to check for duplicates
		existingEnv, _ := os.ReadFile(".env")
		existingEnvStr := string(existingEnv)

		// Parse existing keys to avoid duplicates
		existingKeys := make(map[string]bool)
		for _, line := range strings.Split(existingEnvStr, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			// Handle "export KEY=VALUE"
			line = strings.TrimPrefix(line, "export ")
			parts := strings.SplitN(line, "=", 2)
			if len(parts) > 0 {
				existingKeys[strings.TrimSpace(parts[0])] = true
			}
		}

		f, err := os.OpenFile(".env", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			fmt.Printf("Error opening .env: %v\n", err)
		} else {
			defer f.Close()
			contentToAppend := ""
			if len(existingEnv) > 0 && !strings.HasSuffix(existingEnvStr, "\n") {
				contentToAppend = "\n"
			}

			for _, line := range linesToAppend {
				parts := strings.SplitN(line, "=", 2)
				key := parts[0]
				if !existingKeys[key] {
					contentToAppend += line + "\n"
					existingKeys[key] = true // Prevent duplicates within this batch
				} else {
					fmt.Printf("Note: %s already exists in .env, skipping.\n", key)
				}
			}

			if contentToAppend != "" {
				if _, err := f.WriteString(contentToAppend); err != nil {
					fmt.Printf("Error writing to .env: %v\n", err)
				} else {
					fmt.Printf("Configuration saved to .env\n")
				}
			}
		}
	}

	// Run Doctor
	runDoctor := false
	err = askOneFunc(&survey.Confirm{
		Message: "Run system check (recac doctor) now?",
		Default: true,
	}, &runDoctor)
	if err != nil {
		return err
	}

	if runDoctor {
		fmt.Println("\nRunning Doctor...")
		runDoctorFunc(cmd, args)
	}

	fmt.Println("\nSetup complete! You are ready to code.")
	return nil
}
