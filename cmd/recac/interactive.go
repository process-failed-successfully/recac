package main

import (
	"fmt"
	"os"
	"os/exec"
	"recac/internal/telemetry"
	"recac/internal/ui"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// interactiveCmd represents the interactive command
var interactiveCmd = &cobra.Command{
	Use:   "interactive [mode]",
	Short: "Start an interactive session with the agent",
	Long: `Starts a TUI (Text User Interface) for interactive communication 
with the autonomous coding agent.
Optional mode argument can be 'chat' (default) or 'dashboard'.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Use Viper as it handles env vars and persistent flags binding
		provider := viper.GetString("provider")
		model := viper.GetString("model")
		mode := "chat" // Default mode
		if len(args) > 0 {
			mode = args[0]
		}
		RunInteractive(provider, model, mode, cmd.Parent().Commands())
	},
}

// RunInteractive starts the interactive TUI session.
func RunInteractive(provider, model, mode string, cobraCmds []*cobra.Command) {
	// Define the loader function that will be injected into the UI package.
	loader := func() ([]ui.Session, error) {
		sm, err := sessionManagerFactory()
		if err != nil {
			return nil, fmt.Errorf("failed to create session manager: %w", err)
		}
		// Fetch all local sessions, don't show remote ones in the TUI for now.
		unifiedSessions, err := getFullSessionList(sm, false, "")
		if err != nil {
			return nil, err
		}

		// Map the unified sessions to the UI's session type.
		uiSessions := make([]ui.Session, len(unifiedSessions))
		for i, us := range unifiedSessions {
			details := fmt.Sprintf(
				"Status: %s\nLocation: %s\nStarted: %s\nDuration: %s\n\nTokens: %d\nCost: $%.6f",
				us.Status,
				us.Location,
				us.StartTime.Format("2006-01-02 15:04:05"),
				time.Since(us.StartTime).Round(time.Second),
				us.Tokens.TotalTokens,
				us.Cost,
			)

			uiSessions[i] = ui.Session{
				Name:      us.Name,
				Status:    us.Status,
				Location:  us.Location,
				StartTime: us.StartTime.Format("15:04:05"),
				Cost:      fmt.Sprintf("$%.4f", us.Cost),
				Details:   details,
			}
		}
		return uiSessions, nil
	}

	// Inject the loader into the UI package.
	ui.SetSessionLoader(loader)

	// Redirect logs to file to avoid TUI corruption
	f, err := os.OpenFile("recac-tui.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		f.Close()
		telemetry.InitLogger(viper.GetBool("verbose"), "recac-tui.log", true)
	}

	var initialModel tea.Model
	if mode == "dashboard" {
		initialModel = ui.NewDashboardModel()
	} else {
		commands := buildSlashCommands(cobraCmds)
		initialModel = ui.NewInteractiveModel(commands, provider, model)
	}

	p := tea.NewProgram(initialModel)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		exit(1)
	}
}

// buildSlashCommands constructs the list of commands for the chat TUI.
func buildSlashCommands(cobraCmds []*cobra.Command) []ui.SlashCommand {
	var commands []ui.SlashCommand

	// Internal commands
	commands = append(commands, ui.SlashCommand{
		Name:        "/quit",
		Description: "Exit the application",
		Action:      func(m *ui.InteractiveModel, args []string) tea.Cmd { return tea.Quit },
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
			return func() tea.Msg { return ui.StatusMsg(ui.GetStatus()) }
		},
	})
	commands = append(commands, ui.SlashCommand{
		Name:        "/dashboard",
		Description: "Switch to the session dashboard view",
		Action: func(m *ui.InteractiveModel, args []string) tea.Cmd {
			return func() tea.Msg {
				return tea.Quit // For now, we quit and the user can relaunch
			}
		},
	})

	// Add dynamic commands from Cobra
	for _, c := range cobraCmds {
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
				return func() tea.Msg {
					exe, err := os.Executable()
					if err != nil {
						exe = "recac" // fallback
					}
					fullArgs := append([]string{cmdName}, args...)
					c := exec.Command(exe, fullArgs...)
					output, err := c.CombinedOutput()
					if err != nil {
						return ui.StatusMsg(fmt.Sprintf("Error: %v\n%s", err, string(output)))
					}
					return ui.StatusMsg(string(output))
				}
			},
		})
	}
	return commands
}

func init() {
	rootCmd.AddCommand(interactiveCmd)
}
