package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"recac/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var menuCmd = &cobra.Command{
	Use:   "menu",
	Short: "Interactive command menu",
	Long:  `Explore and execute recac commands interactively.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		commands := collectCommands(rootCmd)

		// Sort commands by path
		sort.Slice(commands, func(i, j int) bool {
			return commands[i].CommandPath() < commands[j].CommandPath()
		})

		var items []ui.MenuItem
		rootName := rootCmd.Name()

		for _, c := range commands {
			// Strip the root command name from display
			name := c.CommandPath()
			if strings.HasPrefix(name, rootName+" ") {
				name = strings.TrimPrefix(name, rootName+" ")
			} else if name == rootName {
				continue // Don't show root command if somehow it got here
			}

			items = append(items, ui.MenuItem{Name: name, Desc: c.Short})
		}

		model := ui.NewMenuModel(items)
		p := tea.NewProgram(model, tea.WithAltScreen())

		m, err := p.Run()
		if err != nil {
			return err
		}

		if menuModel, ok := m.(ui.MenuModel); ok && menuModel.Selected != "" {
			selectedCmd := menuModel.Selected

			// If user selected "menu", just return to avoid infinite loop
			if selectedCmd == "menu" {
				return nil
			}

			fmt.Printf("Executing: %s\n", selectedCmd)

			cmdArgs := strings.Fields(selectedCmd)

			exe, err := os.Executable()
			if err != nil {
				return err
			}

			c := exec.Command(exe, cmdArgs...)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(menuCmd)
}

func collectCommands(root *cobra.Command) []*cobra.Command {
	var commands []*cobra.Command
	for _, c := range root.Commands() {
		if c.Hidden {
			continue
		}
		// If it is runnable, add it
		if c.Runnable() {
			commands = append(commands, c)
		}

		// Recurse
		commands = append(commands, collectCommands(c)...)
	}
	return commands
}
