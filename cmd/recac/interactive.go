package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"recac/internal/runner"
	"recac/internal/telemetry"
	"recac/internal/ui"

	"github.com/AlecAivazis/survey/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// interactiveCmd represents the interactive command
var interactiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Start an interactive session with the agent",
	Long: `Starts a TUI (Text User Interface) for interactive communication 
with the autonomous coding agent.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Use Viper as it handles env vars and persistent flags binding
		provider := viper.GetString("provider")
		model := viper.GetString("model")
		RunInteractive(provider, model)
	},
}

// RunInteractive starts the interactive TUI session.
func RunInteractive(provider, model string) {
	// Redirect logs to file to avoid TUI corruption
	// We do this by re-initializing the logger
	f, err := os.OpenFile("recac-tui.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		f.Close() // Close immediately to release for InitLogger
		// We can't easily swap the global logger sink in the provided slog setup without a helper,
		// but we can try to re-init it.
		// A cleaner way would be provided by telemetry package, but for now re-init:
		telemetry.InitLogger(viper.GetBool("verbose"), "recac-tui.log", true)
		// Also silence standard fmt output if any non-logger prints exist?
		// ideally we just rely on logger redirection.
	}

	// Build commands list from rootCmd
	var commands []ui.SlashCommand

	// Internal commands
	commands = append(commands, ui.SlashCommand{
		Name:        "/quit",
		Description: "Exit the application",
		Action: func(m *ui.InteractiveModel, args []string) tea.Cmd {
			return tea.Quit
		},
	})

	commands = append(commands, ui.SlashCommand{
		Name:        "/clear",
		Description: "Clear conversation history",
		Action: func(m *ui.InteractiveModel, args []string) tea.Cmd {
			m.ClearHistory()
			return nil
		},
	})

	commands = append(commands, ui.SlashCommand{
		Name:        "/status",
		Description: "Show RECAC status",
		Action: func(m *ui.InteractiveModel, args []string) tea.Cmd {
			return func() tea.Msg {
				return ui.StatusMsg(ui.GetStatus())
			}
		},
	})

	// Add dynamic commands from Cobra
	for _, c := range rootCmd.Commands() {
		if c.Name() == "interactive" || c.Name() == "help" || c.Name() == "completion" {
			continue
		}

		cmdName := c.Name()
		slashName := "/" + cmdName
		desc := c.Short

		commands = append(commands, ui.SlashCommand{
			Name:        slashName,
			Description: desc,
			Action: func(m *ui.InteractiveModel, args []string) tea.Cmd {
				// This Cmd function will be executed by the Bubble Tea runtime.
				return func() tea.Msg {
					// Find the executable path.
					exe, err := os.Executable()
					if err != nil {
						exe = "recac" // fallback to assuming recac is in PATH.
					}

					// Prepare the command with its arguments.
					fullArgs := append([]string{cmdName}, args...)
					c := exec.Command(exe, fullArgs...)

					// Execute the command and capture its combined stdout and stderr.
					output, err := c.CombinedOutput()
					if err != nil {
						// If the command fails, prepend the error to the output.
						return ui.StatusMsg(fmt.Sprintf("Error executing command '%s': %v\n%s", cmdName, err, string(output)))
					}

					// On success, return the output to be displayed in the TUI.
					return ui.StatusMsg(string(output))
				}
			},
		})
	}

	p := tea.NewProgram(ui.NewInteractiveModel(commands, provider, model))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		exit(1)
	}
}

func init() {
	rootCmd.AddCommand(interactiveCmd)
}

var surveyAskOne = survey.AskOne

// selectSession prompts the user to select a session based on a filter.
func selectSession(sm ISessionManager, filter func(*runner.SessionState) bool, message string) (string, error) {
	sessions, err := sm.ListSessions()
	if err != nil {
		return "", fmt.Errorf("could not list sessions: %w", err)
	}

	var selectableSessions []string
	for _, s := range sessions {
		if filter(s) {
			selectableSessions = append(selectableSessions, s.Name)
		}
	}

	if len(selectableSessions) == 0 {
		return "", errors.New("no matching sessions found")
	}

	var selectedSession string
	prompt := &survey.Select{
		Message: message,
		Options: selectableSessions,
	}
	// Use WithStdio to ensure it works in various environments
	if err := surveyAskOne(prompt, &selectedSession, survey.WithStdio(os.Stdin, os.Stderr, os.Stderr)); err != nil {
		if err.Error() == "interrupt" {
			return "", errors.New("operation cancelled by user")
		}
		return "", fmt.Errorf("failed to get user input: %w", err)
	}
	return selectedSession, nil
}

// interactiveSessionSelect prompts the user to select a session with a specific status.
func interactiveSessionSelect(sm ISessionManager, status, message string) (string, error) {
	filter := func(s *runner.SessionState) bool {
		return s.Status == status
	}
	session, err := selectSession(sm, filter, message)
	if err != nil && err.Error() == "no matching sessions found" {
		return "", fmt.Errorf("no sessions with status '%s' to select", status)
	}
	return session, err
}

// interactiveSelectRestartableSession prompts the user to select from a list of non-running sessions.
func interactiveSelectRestartableSession(sm ISessionManager, message string) (string, error) {
	filter := func(s *runner.SessionState) bool {
		return s.Status != "running"
	}
	session, err := selectSession(sm, filter, message)
	if err != nil && err.Error() == "no matching sessions found" {
		return "", errors.New("no restartable sessions found")
	}
	return session, err
}
