package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// BlameLine represents a single line of code with blame metadata.
type BlameLine struct {
	LineNo  int
	Content string
	SHA     string
	Author  string
	Date    string
	Summary string
}

// BlameModel is the Bubble Tea model for the interactive blame view.
type BlameModel struct {
	Lines         []BlameLine
	Cursor        int // Current selected line index
	ViewportStart int // Index of the first visible line
	Height        int
	Width         int

	// Detail View (Diff/Explanation)
	detailsViewport viewport.Model
	viewingDetails  bool
	statusMessage   string

	// Callbacks
	fetchDiffFunc func(sha string) (string, error)
	explainFunc   func(sha string) (string, error)
}

type blameDiffMsg struct {
	content string
	err     error
}

type blameAnalysisMsg struct {
	result string
	err    error
}

// NewBlameModel creates a new model.
func NewBlameModel(lines []BlameLine, fetchDiff func(string) (string, error), explain func(string) (string, error)) BlameModel {
	return BlameModel{
		Lines:           lines,
		fetchDiffFunc:   fetchDiff,
		explainFunc:     explain,
		detailsViewport: viewport.New(0, 0),
	}
}

func (m BlameModel) Init() tea.Cmd {
	return nil
}

func (m BlameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.detailsViewport.Width = msg.Width
		m.detailsViewport.Height = msg.Height - 1 // Leave room for header/status

	case tea.KeyMsg:
		if m.viewingDetails {
			switch msg.String() {
			case "q", "esc":
				m.viewingDetails = false
				m.statusMessage = ""
				return m, nil
			default:
				m.detailsViewport, cmd = m.detailsViewport.Update(msg)
				return m, cmd
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
				if m.Cursor < m.ViewportStart {
					m.ViewportStart = m.Cursor
				}
			}
		case "down", "j":
			if m.Cursor < len(m.Lines)-1 {
				m.Cursor++
				if m.Cursor >= m.ViewportStart+m.contentHeight() {
					m.ViewportStart = m.Cursor - m.contentHeight() + 1
				}
			}
		case "enter":
			if m.fetchDiffFunc != nil && len(m.Lines) > 0 {
				sha := m.Lines[m.Cursor].SHA
				m.statusMessage = fmt.Sprintf("Fetching diff for %s...", sha)
				return m, func() tea.Msg {
					diff, err := m.fetchDiffFunc(sha)
					return blameDiffMsg{content: diff, err: err}
				}
			}
		case "e":
			if m.explainFunc != nil && len(m.Lines) > 0 {
				sha := m.Lines[m.Cursor].SHA
				m.statusMessage = fmt.Sprintf("Asking AI to explain %s...", sha)
				return m, func() tea.Msg {
					exp, err := m.explainFunc(sha)
					return blameAnalysisMsg{result: exp, err: err}
				}
			}
		}

	case blameDiffMsg:
		m.statusMessage = ""
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.viewingDetails = true
			m.detailsViewport.SetContent(msg.content)
			m.detailsViewport.GotoTop()
		}

	case blameAnalysisMsg:
		m.statusMessage = ""
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.viewingDetails = true
			m.detailsViewport.SetContent(msg.result)
			m.detailsViewport.GotoTop()
		}
	}

	return m, nil
}

func (m BlameModel) contentHeight() int {
	// Height reserved for content (total height - header - status/footer)
	// We'll use a split view: top 70% code, bottom 30% blame info?
	// Or just a footer.
	// Let's say footer is 4 lines.
	h := m.Height - 5
	if h < 1 {
		return 1
	}
	return h
}

func (m BlameModel) View() string {
	if m.viewingDetails {
		header := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("Commit Details (q to close)")
		return fmt.Sprintf("%s\n%s", header, m.detailsViewport.View())
	}

	if len(m.Lines) == 0 {
		return "No lines to blame."
	}

	// Render Code View
	var sb strings.Builder

	end := m.ViewportStart + m.contentHeight()
	if end > len(m.Lines) {
		end = len(m.Lines)
	}

	lineNoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Width(4).Align(lipgloss.Right).MarginRight(1)
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	for i := m.ViewportStart; i < end; i++ {
		line := m.Lines[i]

		lnStr := fmt.Sprintf("%d", line.LineNo)
		content := line.Content

		// Tab expansion
		content = strings.ReplaceAll(content, "\t", "    ")

		renderedLine := ""
		if i == m.Cursor {
			renderedLine = fmt.Sprintf("%s %s", lineNoStyle.Render(lnStr), cursorStyle.Render(content))
		} else {
			renderedLine = fmt.Sprintf("%s %s", lineNoStyle.Render(lnStr), normalStyle.Render(content))
		}

		sb.WriteString(renderedLine + "\n")
	}

	// Fill remaining space if any
	renderedLinesCount := end - m.ViewportStart
	remaining := m.contentHeight() - renderedLinesCount
	if remaining > 0 {
		sb.WriteString(strings.Repeat("\n", remaining))
	}

	// Separator
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("236")).Render(strings.Repeat("─", m.Width)) + "\n")

	// Blame Info Panel
	current := m.Lines[m.Cursor]

	infoStyle := lipgloss.NewStyle().Padding(0, 1)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	info := fmt.Sprintf(
		"%s %s\n%s %s\n%s %s\n%s %s",
		labelStyle.Render("Commit:"), valueStyle.Render(current.SHA[:8]),
		labelStyle.Render("Author:"), valueStyle.Render(current.Author),
		labelStyle.Render("Date:  "), valueStyle.Render(current.Date),
		labelStyle.Render("Summary:"), valueStyle.Render(current.Summary),
	)
	sb.WriteString(infoStyle.Render(info))

	// Status Bar
	if m.statusMessage != "" {
		sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(m.statusMessage))
	} else {
		sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Use j/k to navigate, Enter for Diff, 'e' for AI Explain"))
	}

	return sb.String()
}
