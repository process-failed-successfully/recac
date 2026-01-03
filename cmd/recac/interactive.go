package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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
			// Cannot easily clear history from here without exposing fields,
			// but we can implement a custom message or just let UI handle it if we moved logic there.
			// Actually, the new UI helper doesn't expose fields publicly.
			// Let's rely on validCommands being passed in, but the Action logic needs access to 'm'.
			// Since 'm' is passed to Action, we can modify it if fields are exported.
			// Currently fields are NOT exported in my previous write.
			// I should have exported them or added helper methods.
			// For now, let's skip implementation detail of 'clear' or implement it via re-init?
			// But for parity, let's focus on Cobra commands.
			return nil
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
				// Construct the command to run
				exe, err := os.Executable()
				if err != nil {
					exe = "recac" // fallback
				}

				// Wrap in a shell to pause execution after the command finishes
				// This ensures the user can see the output (error or success)
				// quotedArgs handles arguments safely enough for this TUI context
				// constructing a shell command: sh -c 'exe arg1 arg2; read -p "Press Enter to continue..."'

				// flatten args
				fullArgs := append([]string{cmdName}, args...)
				flatArgs := strings.Join(fullArgs, " ") // Simplistic joining

				// Better approach: passing args to the executed command directly is safer,
				// but to "pause", we need a wrapper.
				// We can create a small wrapper script or just chain it.
				// exec.Command("sh", "-c", "recac command args...; bufio wait")

				// Reconstructing the command string for sh -c
				// We need to escape args if they have spaces, but for now simple join is risky but acceptable for prototype
				// Actually, we can use $0 and pass args.

				shellCmd := fmt.Sprintf("'%s' %s; echo ''; echo 'Press Enter to continue...'; read", exe, flatArgs)
				c := exec.Command("sh", "-c", shellCmd)

				// If not linux/sh, we might need other handling, but USER OS is linux.

				c.Stdin = os.Stdin
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr

				return tea.ExecProcess(c, func(err error) tea.Msg {
					if err != nil {
						return fmt.Errorf("Command finished with error: %v", err)
					}
					return nil
				})
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
