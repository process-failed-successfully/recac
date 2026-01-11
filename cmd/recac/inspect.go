package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/runner"

	"github.com
/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect [SESSION_NAME]",
	Short: "Display detailed information about a session",
	Long:  `Inspect provides a detailed summary of a specific session, including its status, metadata, token usage, cost, and recent log output.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]
		sm, err := runner.NewSessionManager()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		session, err := sm.GetSession(sessionName)
		if err != nil {
			return err
		}

		// Display session details in a structured format
		cmd.Println(c.Bold("Session Details"))
		cmd.Println("----------------")
		cmd.Printf("Name: %s\n", session.Name)
		cmd.Printf("Status: %s\n", session.Status)
		cmd.Printf("Start Time: %s\n", session.StartTime.Format("2006-01-02 15:04:05"))
		if !session.EndTime.IsZero() {
			cmd.Printf("End Time: %s\n", session.EndTime.Format("2006-01-02 15:04:05"))
			cmd.Printf("Duration: %s\n", session.EndTime.Sub(session.StartTime).Round(time.Second))
		}
		if session.Error != "" {
			cmd.Printf("Error: %s\n", c.Red(session.Error))
		}
		cmd.Println("")

		// Display Cost and Token Information
		agentState, err := loadAgentState(sm.GetSessionDir(sessionName))
		if err == nil {
			cost, costOk := agent.CalculateCost(agentState.Model, agentState.TokenUsage)
			cmd.Println(c.Bold("Token & Cost Details"))
			cmd.Println("--------------------")
			cmd.Printf("Model: %s\n", agentState.Model)
			cmd.Printf("Prompt Tokens: %d\n", agentState.TotalPromptTokens)
			cmd.Printf("Response Tokens: %d\n", agentState.TotalResponseTokens)
			if costOk {
				cmd.Printf("Estimated Cost: %s\n", c.Green(fmt.Sprintf("$%.4f", cost)))
			} else {
				cmd.Printf("Estimated Cost: %s\n", c.Yellow("Not available"))
			}
			cmd.Println("")
		}

		// Display last 10 lines of logs
		logPath := sm.GetSessionLogPath(sessionName)
		if _, err := os.Stat(logPath); err == nil {
			logs, err := readLastNLines(logPath, 10)
			if err != nil {
				return fmt.Errorf("failed to read logs: %w", err)
			}
			cmd.Println(c.Bold("Recent Logs"))
			cmd.Println("-----------")
			cmd.Println(logs)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(inspectCmd)
}

func readLastNLines(filePath string, n int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// This is a simple implementation. For very large files, a more
	// efficient approach (e.g., seeking to the end and reading backwards)
	// would be better.
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	start := len(lines) - n
	if start < 0 {
		start = 0
	}

	return strings.Join(lines[start:], "\n"), nil
}
