package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type attachDashboardModel struct {
	sessionName string
	logFile     string
	reader      *bufio.Reader
	file        *os.File

	session     *runner.SessionState
	agentState  *agent.State
	gitDiffStat string
	err         error

	viewport    viewport.Model
	ready       bool
	logContent  strings.Builder // Buffer for logs

	width  int
	height int
}

type logMsg string
type logReadErrorMsg struct { err error }

func (e logReadErrorMsg) Error() string { return e.err.Error() }

func NewAttachDashboardModel(sessionName, logFile string) *attachDashboardModel {
	return &attachDashboardModel{
		sessionName: sessionName,
		logFile:     logFile,
	}
}

func (m *attachDashboardModel) Init() tea.Cmd {
	return tea.Batch(
		refreshStatusCmd(m.sessionName),
		tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
			return statusTickMsg(t)
		}),
		waitForLog(m.reader),
	)
}

func (m *attachDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		statusHeight := 12 // Adjusted estimate

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-statusHeight)
			m.viewport.YPosition = statusHeight
			m.viewport.HighPerformanceRendering = false
			m.viewport.SetContent(m.logContent.String())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - statusHeight
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "down", "j":
			m.viewport.LineDown(1)
		case "up", "k":
			m.viewport.LineUp(1)
		case "pgdown":
			m.viewport.ViewDown()
		case "pgup":
			m.viewport.ViewUp()
		case "end":
			m.viewport.GotoBottom()
		}

	case statusTickMsg:
		cmds = append(cmds,
			refreshStatusCmd(m.sessionName),
			tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
				return statusTickMsg(t)
			}),
		)

	case statusRefreshedMsg:
		m.session = msg.session
		m.agentState = msg.agentState
		m.gitDiffStat = msg.gitDiffStat

	case logMsg:
		// Append to content buffer
		m.logContent.WriteString(string(msg))

		// Update viewport content
		// Only auto-scroll if we were already at the bottom
		atBottom := m.viewport.AtBottom()

		m.viewport.SetContent(m.logContent.String())

		if atBottom {
			m.viewport.GotoBottom()
		}

		// Continue reading logs
		cmds = append(cmds, waitForLog(m.reader))

	case logReadErrorMsg:
		m.err = msg.err
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *attachDashboardModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}
	if !m.ready {
		return "Initializing Dashboard..."
	}

	var s strings.Builder

	// --- Header ---
	s.WriteString(statusTitleStyle.Render(fmt.Sprintf(" RECAC Session: %s (LIVE)", m.sessionName)) + "\n")

	// --- Status Section ---
	if m.session != nil {
		// We reuse the functions from status_dashboard.go (they are in the same package)
		// Assuming they are exported or accessible.
		// They are in the same package `ui`, so yes.
		s.WriteString(renderSessionInfo(m.session, m.agentState))
	} else {
		s.WriteString("Loading session info...\n")
	}

	// --- Separator ---
	s.WriteString(sectionStyle.Render("--- Live Logs (tail -f) ---") + "\n")

	// --- Viewport ---
	s.WriteString(m.viewport.View())

	return s.String()
}

// waitForLog waits for the next line from the reader
func waitForLog(reader *bufio.Reader) tea.Cmd {
	return func() tea.Msg {
		if reader == nil {
			return logReadErrorMsg{fmt.Errorf("log reader is nil")}
		}
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					if line != "" {
						return logMsg(line)
					}
					time.Sleep(200 * time.Millisecond)
					continue
				}
				return logReadErrorMsg{err}
			}
			return logMsg(line)
		}
	}
}

// StartAttachDashboard starts the TUI dashboard for a specific session with live logs
func StartAttachDashboard(sessionName, logFile string) error {
	f, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	// Note: The file will be closed when the program quits in KeyMsg handler
	// or we can defer close here but Run() blocks, so defer is okay?
	// If we defer close here, it closes when Run() returns. That's fine.
	defer f.Close()

	m := NewAttachDashboardModel(sessionName, logFile)
	m.file = f
	m.reader = bufio.NewReader(f)

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
