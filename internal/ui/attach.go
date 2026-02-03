package ui

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AttachDashboardModel struct {
	viewport    viewport.Model
	logFilePath string
	offset      int64
	entries     []LogEntry
	autoScroll  bool
	sessionName string
	status      string // e.g. "Running", "Stopped", "Error"
	err         error
	width       int
	height      int
	ready       bool
}

type attachTickMsg time.Time

type fileUpdateMsg struct {
	newContent []byte
	newOffset  int64
	err        error
}

func NewAttachDashboardModel(sessionName, logFilePath, status string) AttachDashboardModel {
	vp := viewport.New(0, 0)
	vp.YPosition = 0

	return AttachDashboardModel{
		viewport:    vp,
		logFilePath: logFilePath,
		sessionName: sessionName,
		status:      status,
		autoScroll:  true,
	}
}

func (m AttachDashboardModel) Init() tea.Cmd {
	return tea.Batch(
		m.initialReadCmd(),
		tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return attachTickMsg(t)
		}),
	)
}

func (m AttachDashboardModel) initialReadCmd() tea.Cmd {
	return func() tea.Msg {
		f, err := os.Open(m.logFilePath)
		if err != nil {
			return fileUpdateMsg{err: err}
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil {
			return fileUpdateMsg{err: err}
		}

		// Read all content initially
		content := make([]byte, stat.Size())
		_, err = f.Read(content)
		if err != nil {
			return fileUpdateMsg{err: err}
		}

		return fileUpdateMsg{
			newContent: content,
			newOffset:  stat.Size(),
		}
	}
}

func (m AttachDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "end":
			m.autoScroll = true
			m.viewport.GotoBottom()
		case "home":
			m.autoScroll = false
			m.viewport.GotoTop()
		case "up", "pgup":
			m.autoScroll = false // User wants to scroll up
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := 2 // Title + Status + Border
		footerHeight := 1 // Help
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}

		// Re-render content to fit new width
		m.updateViewportContent()

	case attachTickMsg:
		// Check for updates
		return m, tea.Batch(
			m.checkFileUpdateCmd(),
			tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
				return attachTickMsg(t)
			}),
		)

	case fileUpdateMsg:
		if msg.err != nil {
			m.err = msg.err
			// Log error to viewport?
			return m, nil
		}

		if len(msg.newContent) > 0 {
			// Parse new lines
			newEntries, err := ParseLogLines(msg.newContent)
			if err == nil {
				m.entries = append(m.entries, newEntries...)
				m.offset = msg.newOffset
				m.updateViewportContent()
				if m.autoScroll {
					m.viewport.GotoBottom()
				}
			}
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m AttachDashboardModel) checkFileUpdateCmd() tea.Cmd {
	return func() tea.Msg {
		f, err := os.Open(m.logFilePath)
		if err != nil {
			return fileUpdateMsg{err: err}
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil {
			return fileUpdateMsg{err: err}
		}

		currentSize := stat.Size()
		if currentSize > m.offset {
			diff := currentSize - m.offset

			// If file was truncated (rotated), start over logic could go here
			// For now assume append-only

			buffer := make([]byte, diff)
			n, err := f.ReadAt(buffer, m.offset)
			if err != nil {
				return fileUpdateMsg{err: err}
			}
			buffer = buffer[:n]

			// Robustness: Only accept complete lines
			lastNewline := bytes.LastIndexByte(buffer, '\n')
			if lastNewline == -1 {
				// No newline found, wait for more data
				return fileUpdateMsg{newOffset: m.offset}
			}

			// Slice up to the last newline
			validContent := buffer[:lastNewline+1]
			newOffset := m.offset + int64(lastNewline+1)

			return fileUpdateMsg{
				newContent: validContent,
				newOffset:  newOffset,
			}
		}

		return fileUpdateMsg{
			newContent: nil,
			newOffset:  m.offset,
		}
	}
}

func (m AttachDashboardModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	header := m.headerView()
	footer := m.footerView()

	return fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), footer)
}

func (m AttachDashboardModel) headerView() string {
	title := attachTitleStyle.Render(fmt.Sprintf(" Session: %s ", m.sessionName))
	statusColor := "205" // Pink
	if m.status == "Running" {
		statusColor = "42" // Green
	} else if m.status == "Error" {
		statusColor = "196" // Red
	}

	status := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor)).Render(m.status)
	line := strings.Repeat("─", max(0, m.width-lipgloss.Width(title)-lipgloss.Width(status)-2))

	return lipgloss.JoinHorizontal(lipgloss.Center, title, line, status)
}

func (m AttachDashboardModel) footerView() string {
	info := attachHelpStyle.Render("Press 'q' to quit • 'end' to auto-scroll • 'up' to pause scroll")
	if m.autoScroll {
		info += lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(" [FOLLOWING]")
	} else {
		info += lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(" [PAUSED]")
	}
	return info
}

var (
	attachTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	attachHelpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

func (m *AttachDashboardModel) updateViewportContent() {
	var sb strings.Builder
	for _, entry := range m.entries {
		sb.WriteString(entry.Content)
		sb.WriteString("\n")
	}
	m.viewport.SetContent(sb.String())
}
