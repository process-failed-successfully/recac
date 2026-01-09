package main

import (
	"fmt"
	"os"
	"os/exec"

	"recac/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// interactiveCmd represents the interactive command
var interactiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Start an interactive session with the agent",
	Long: `Starts a TUI (Text User Interface) for interactive communication 
with the autonomous coding agent.`,
	Run: func(cmd *cobra.Command, args []string) {
		RunInteractive()
	},
}

// RunInteractive starts the interactive TUI session.
func RunInteractive() {
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
				status, err := ui.GetStatus()
				if err != nil {
					return ui.ErrorMsg(err)
				}
				return ui.StatusMsg(status)
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

	p := tea.NewProgram(ui.NewInteractiveModel(commands))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		exit(1)
	}
}

func init() {
	rootCmd.AddCommand(interactiveCmd)
}
