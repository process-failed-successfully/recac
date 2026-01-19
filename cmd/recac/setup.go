package main

import (
	"fmt"
	"os"
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

// setupCmd represents the setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactively set up RECAC configuration",
	Long:  `Runs an interactive wizard to configure RECAC settings, including provider, model, and API keys.`,
	RunE:  runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	fmt.Println("Welcome to RECAC Setup!")
	fmt.Println("-----------------------")

	answers := struct {
		Provider     string
		Model        string
		ApiKey       string
		SaveToEnv    bool
		EnableSlack  bool
		SlackChannel string
		SlackToken   string
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

	// 4. Notifications
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
	if answers.EnableSlack {
		viper.Set("notifications.slack.enabled", true)
		viper.Set("notifications.slack.channel", answers.SlackChannel)
	}

	// Write to config.yaml
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = "config.yaml"
	}

	if err := viper.WriteConfigAs(configFile); err != nil {
		fmt.Printf("Warning: Could not write %s: %v\n", configFile, err)
	} else {
		fmt.Printf("Configuration saved to %s\n", configFile)
	}

	// Write to .env
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

		newEnvLine := fmt.Sprintf("%s=%s", envKey, answers.ApiKey)
		slackEnvLine := ""
		if answers.EnableSlack && answers.SlackToken != "" {
			slackEnvLine = fmt.Sprintf("SLACK_BOT_USER_TOKEN=%s", answers.SlackToken)
		}

		// Read existing .env to check for duplicates
		existingEnv, _ := os.ReadFile(".env")
		existingEnvStr := string(existingEnv)

		var linesToAppend []string

		if !strings.Contains(existingEnvStr, fmt.Sprintf("%s=", envKey)) {
			linesToAppend = append(linesToAppend, newEnvLine)
		} else {
			fmt.Printf("Note: %s already exists in .env, skipping.\n", envKey)
		}

		if slackEnvLine != "" {
			if !strings.Contains(existingEnvStr, "SLACK_BOT_USER_TOKEN=") {
				linesToAppend = append(linesToAppend, slackEnvLine)
			} else {
				fmt.Println("Note: SLACK_BOT_USER_TOKEN already exists in .env, skipping.")
			}
		}

		if len(linesToAppend) > 0 {
			f, err := os.OpenFile(".env", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				fmt.Printf("Error opening .env: %v\n", err)
			} else {
				defer f.Close()
				contentToAppend := ""
				if len(existingEnv) > 0 && !strings.HasSuffix(existingEnvStr, "\n") {
					contentToAppend = "\n"
				}
				contentToAppend += strings.Join(linesToAppend, "\n") + "\n"

				if _, err := f.WriteString(contentToAppend); err != nil {
					fmt.Printf("Error writing to .env: %v\n", err)
				} else {
					fmt.Println("Secrets saved to .env")
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
