package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
	"github.com/kballard/go-shellquote"
)

// DevExecCmdFactory allows mocking exec.Command, can be overwritten by tests
var DevExecCmdFactory = exec.Command

var (
	devTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	devStatusStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1)

	devSuccessStyle = devStatusStyle.Copy().Foreground(lipgloss.Color("#04B575"))
	devErrorStyle   = devStatusStyle.Copy().Foreground(lipgloss.Color("#FF0000"))
	devRunningStyle = devStatusStyle.Copy().Foreground(lipgloss.Color("#FFFF00"))
	devIdleStyle    = devStatusStyle.Copy().Foreground(lipgloss.Color("#AAAAAA"))
)

type DevDashboardModel struct {
	watcher    *fsnotify.Watcher
	viewport   viewport.Model
	Command    string
	WatchDir   string
	Extensions []string
	Recursive  bool
	Debounce   time.Duration

	// State
	Output       string
	Status       string // "Idle", "Running", "Success", "Failed"
	LastRun      time.Time
	LastDuration time.Duration
	Err          error
	Ready        bool
	Width        int
	Height       int

	// Internal for concurrency/debounce
	pendingRun  bool
	debounceTag int
}

type FileChangeMsg struct{ Event fsnotify.Event }
type FileErrorMsg struct{ Err error }
type CommandFinishedMsg struct {
	Output   string
	Err      error
	Duration time.Duration
}
type RunCommandMsg struct{}
type DebounceMsg struct{ tag int }

func NewDevDashboardModel(cmd, dir string, exts []string, recursive bool, debounce time.Duration) (DevDashboardModel, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return DevDashboardModel{}, err
	}

	// Add initial watch
	if recursive {
		if err := addRecursiveWatch(watcher, dir); err != nil {
			return DevDashboardModel{}, err
		}
	} else {
		if err := watcher.Add(dir); err != nil {
			return DevDashboardModel{}, err
		}
	}

	return DevDashboardModel{
		watcher:    watcher,
		Command:    cmd,
		WatchDir:   dir,
		Extensions: exts,
		Recursive:  recursive,
		Debounce:   debounce,
		Status:     "Idle",
	}, nil
}

func (m DevDashboardModel) Init() tea.Cmd {
	return tea.Batch(
		waitForFileChange(m.watcher),
		func() tea.Msg { return RunCommandMsg{} }, // Initial run
	)
}

func (m DevDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		headerHeight := 3
		footerHeight := 3
		verticalMarginHeight := headerHeight + footerHeight

		if !m.Ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.Ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}
		m.Width = msg.Width
		m.Height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "r":
			return m.triggerRun()
		}
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)

	case FileChangeMsg:
		// Filter events (Write, Create, Rename)
		if msg.Event.Op&fsnotify.Write == fsnotify.Write || msg.Event.Op&fsnotify.Create == fsnotify.Create || msg.Event.Op&fsnotify.Rename == fsnotify.Rename {
			// Check extension
			if shouldTrigger(msg.Event.Name, m.Extensions) {
				m.debounceTag++
				tag := m.debounceTag
				return m, tea.Batch(
					tea.Tick(m.Debounce, func(_ time.Time) tea.Msg {
						return DebounceMsg{tag: tag}
					}),
					waitForFileChange(m.watcher),
				)
			}
		}

		// If new directory created, add to watcher
		if m.Recursive && msg.Event.Op&fsnotify.Create == fsnotify.Create {
			fi, err := os.Stat(msg.Event.Name)
			if err == nil && fi.IsDir() {
				m.watcher.Add(msg.Event.Name)
			}
		}

		return m, waitForFileChange(m.watcher)

	case FileErrorMsg:
		m.Err = msg.Err
		return m, waitForFileChange(m.watcher)

	case DebounceMsg:
		if msg.tag == m.debounceTag {
			return m.triggerRun()
		}

	case RunCommandMsg:
		return m.triggerRun()

	case CommandFinishedMsg:
		m.LastRun = time.Now()
		m.LastDuration = msg.Duration
		m.Output = msg.Output
		m.viewport.SetContent(m.Output)
		if msg.Err != nil {
			m.Status = "Failed"
			m.viewport.GotoBottom()
		} else {
			m.Status = "Success"
		}

		if m.pendingRun {
			m.pendingRun = false
			m.Status = "Running"
			return m, executeDevCommand(m.Command)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m DevDashboardModel) triggerRun() (tea.Model, tea.Cmd) {
	if m.Status == "Running" {
		m.pendingRun = true
		return m, nil
	}
	m.Status = "Running"
	return m, executeDevCommand(m.Command)
}

func (m DevDashboardModel) View() string {
	if !m.Ready {
		return "\n  Initializing..."
	}

	header := m.headerView()
	footer := m.footerView()

	return fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), footer)
}

func (m DevDashboardModel) headerView() string {
	title := devTitleStyle.Render(fmt.Sprintf("Dev Mode: %s", m.Command))
	line := strings.Repeat("─", devMax(0, m.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m DevDashboardModel) footerView() string {
	var status string
	switch m.Status {
	case "Running":
		status = devRunningStyle.Render("RUNNING")
	case "Success":
		status = devSuccessStyle.Render("SUCCESS")
	case "Failed":
		status = devErrorStyle.Render("FAILED")
	default:
		status = devIdleStyle.Render("IDLE")
	}

	info := fmt.Sprintf("Last: %s (%s)", m.LastRun.Format("15:04:05"), m.LastDuration)
	help := "r: rerun • q: quit"

	if m.pendingRun {
		status += " (Pending...)"
	}

	statusBlock := fmt.Sprintf("%s  %s  %s", status, info, help)
	line := strings.Repeat("─", devMax(0, m.Width-lipgloss.Width(statusBlock)))

	return lipgloss.JoinHorizontal(lipgloss.Center, line, statusBlock)
}

// Commands

func waitForFileChange(watcher *fsnotify.Watcher) tea.Cmd {
	return func() tea.Msg {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			return FileChangeMsg{Event: event}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return FileErrorMsg{Err: err}
		}
	}
}

func executeDevCommand(cmdStr string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		parts, err := shellquote.Split(cmdStr)
		if err != nil {
			return CommandFinishedMsg{
				Output:   fmt.Sprintf("Error parsing command: %v", err),
				Err:      err,
				Duration: time.Since(start),
			}
		}

		if len(parts) == 0 {
			return CommandFinishedMsg{Duration: time.Since(start)}
		}

		c := DevExecCmdFactory(parts[0], parts[1:]...)
		// We could use c.StdoutPipe() here for streaming, but keeping it simple for now
		out, err := c.CombinedOutput()

		return CommandFinishedMsg{
			Output:   string(out),
			Err:      err,
			Duration: time.Since(start),
		}
	}
}

// Helpers

func addRecursiveWatch(watcher *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			if name == "node_modules" || name == "vendor" || name == "dist" || name == "build" || name == "target" || name == "bin" {
				return filepath.SkipDir
			}
			return watcher.Add(path)
		}
		return nil
	})
}

func shouldTrigger(path string, exts []string) bool {
	if len(exts) == 0 {
		return true
	}
	for _, ext := range exts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

func devMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
