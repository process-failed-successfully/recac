package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/charmbracelet/bubbles/timer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	focusDuration time.Duration
	focusTask     string
	focusMusic    bool
	focusDND      bool
)

var startFocusTUIFunc = func(m tea.Model) error {
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

var focusCmd = &cobra.Command{
	Use:   "focus",
	Short: "Start a focus session (Pomodoro timer)",
	Long: `Starts a focus session with a countdown timer.
Optionally plays lofi music and attempts to toggle Do Not Disturb (macOS only).`,
	RunE: runFocus,
}

func init() {
	if rootCmd != nil {
		rootCmd.AddCommand(focusCmd)
	}
	focusCmd.Flags().DurationVarP(&focusDuration, "duration", "d", 25*time.Minute, "Duration of the focus session")
	focusCmd.Flags().StringVarP(&focusTask, "task", "t", "", "Task to focus on")
	focusCmd.Flags().BoolVarP(&focusMusic, "music", "m", false, "Play lofi music (opens YouTube)")
	focusCmd.Flags().BoolVar(&focusDND, "dnd", false, "Toggle Do Not Disturb (macOS only, via Shortcuts)")
}

func runFocus(cmd *cobra.Command, args []string) error {
	if focusTask == "" {
		fmt.Print("What is your goal for this session? ")
		// Use Fscanln on InOrStdin if possible to support piping/testing
		fmt.Scanln(&focusTask)
	}

	if focusMusic {
		if err := startMusic(); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to start music: %v\n", err)
		} else {
			fmt.Println("🎧 Music started.")
		}
	}

	if focusDND {
		if err := toggleDND(true); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to enable DND: %v\n", err)
		} else {
			fmt.Println("🔕 Do Not Disturb enabled (if Shortcut exists).")
			defer toggleDND(false) // Disable on exit
		}
	}

	m := initialFocusModel(focusDuration, focusTask)

	if err := startFocusTUIFunc(m); err != nil {
		return fmt.Errorf("error running focus timer: %w", err)
	}

	fmt.Println("\n🎉 Session complete!")
	return nil
}

type focusModel struct {
	timer    timer.Model
	task     string
	finished bool
}

func initialFocusModel(duration time.Duration, task string) focusModel {
	return focusModel{
		timer:    timer.NewWithInterval(duration, time.Second),
		task:     task,
		finished: false,
	}
}

func (m focusModel) Init() tea.Cmd {
	return m.timer.Init()
}

func (m focusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		}
	case timer.TimeoutMsg:
		m.finished = true
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.timer, cmd = m.timer.Update(msg)
	return m, cmd
}

func (m focusModel) View() string {
	if m.finished {
		return "Done!"
	}

	s := fmt.Sprintf("\n🎯 Focus: %s\n", m.task)
	s += fmt.Sprintf("⏳ %s\n", m.timer.View())
	s += "\n(q to quit)\n"
	return s
}

func startMusic() error {
	url := "https://www.youtube.com/watch?v=jfKfPfyJRdk" // Lofi Girl
	var cmdName string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmdName = "open"
		args = []string{url}
	case "linux":
		cmdName = "xdg-open"
		args = []string{url}
	case "windows":
		cmdName = "cmd"
		args = []string{"/c", "start", url}
	default:
		return fmt.Errorf("unsupported OS for music: %s", runtime.GOOS)
	}

	return execCommand(cmdName, args...).Start()
}

func toggleDND(enable bool) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("DND is only supported on macOS")
	}

	action := "On"
	if !enable {
		action = "Off"
	}

	// Try to run a Shortcut named "Turn On Do Not Disturb" or "Turn Off Do Not Disturb"
	// This is the standard way on macOS Monterey+ if configured.
	// If it fails, we fallback to notification.

	shortcutName := fmt.Sprintf("Turn %s Do Not Disturb", action)
	err := execCommand("shortcuts", "run", shortcutName).Run()

	if err != nil {
		// Fallback to notification
		script := fmt.Sprintf(`display notification "Focus Mode %s (DND Shortcut not found)" with title "RECAC"`, action)
		return execCommand("osascript", "-e", script).Run()
	}

	return nil
}
