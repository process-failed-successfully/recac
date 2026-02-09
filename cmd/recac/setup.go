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

var runDoctorFunc = func(cmd *cobra.Command, args []string) {
	if doctorCmd.Run != nil {
		doctorCmd.Run(cmd, args)
	} else if doctorCmd.RunE != nil {
		// RunE returns an error, but here we just ignore it as per original logic?
		// Or print it?
		if err := doctorCmd.RunE(cmd, args); err != nil {
			fmt.Printf("Doctor check failed: %v\n", err)
		}
	}
}

var askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
	return survey.AskOne(p, response, opts...)
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactively set up RECAC configuration",
	Long:  `Runs an interactive wizard to configure RECAC settings, including provider, model, API keys, and Jira integration.`,
	RunE:  runSetup,
}

var envFilePath = ".env"

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	fmt.Println("Welcome to RECAC Setup Wizard!")
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
		Options: []string{"openai", "anthropic", "gemini", "openrouter", "mock"},
		Default: "openai",
	}, &answers.Provider)
	if err != nil {
		return err
	}

	// 2. Model Configuration
	defaultModel := "gpt-4o"
	if answers.Provider == "anthropic" {
		defaultModel = "claude-3-opus-20240229"
	} else if answers.Provider == "gemini" {
		defaultModel = "gemini-pro"
	}

	err = askOneFunc(&survey.Input{
		Message: "Enter the Model name:",
		Default: defaultModel,
	}, &answers.Model)
	if err != nil {
		return err
	}

	// 3. API Key
	err = askOneFunc(&survey.Input{
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
			Default: "#alerts",
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

	// 6. Run Doctor Check?
	var runDoc bool
	err = askOneFunc(&survey.Confirm{
		Message: "Run system check (recac doctor) now?",
		Default: true,
	}, &runDoc)
	if err != nil {
		return err
	}

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
		return fmt.Errorf("failed to write config: %w", err)
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
		case "openai":
			envKey = "OPENAI_API_KEY"
		case "anthropic":
			envKey = "ANTHROPIC_API_KEY"
		case "gemini":
			envKey = "GEMINI_API_KEY"
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
		existingEnv, _ := os.ReadFile(envFilePath)
		existingEnvStr := string(existingEnv)

		f, err := os.OpenFile(envFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("error opening %s: %w", envFilePath, err)
		}
		defer f.Close()

		contentToAppend := ""
		if len(existingEnv) > 0 && !strings.HasSuffix(existingEnvStr, "\n") {
			contentToAppend = "\n"
		}

		for _, line := range linesToAppend {
			parts := strings.SplitN(line, "=", 2)
			key := parts[0]
			// Check if key exists by searching for "\nKEY=" to ensure full match
			// We prepend "\n" to existingEnvStr so the first line is also checked correctly
			if !strings.Contains("\n"+existingEnvStr, "\n"+key+"=") {
				contentToAppend += line + "\n"
			} else {
				fmt.Printf("Note: %s already exists in %s, skipping.\n", key, envFilePath)
			}
		}

		if contentToAppend != "" {
			if _, err := f.WriteString(contentToAppend); err != nil {
				return fmt.Errorf("error writing to %s: %w", envFilePath, err)
			} else {
				fmt.Printf("Secrets saved to %s\n", envFilePath)
			}
		}
	}

	fmt.Printf("Configuration saved to %s\n", configFile)

	if runDoc {
		runDoctorFunc(cmd, args)
	}

	return nil
}
