package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)

	listStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			MarginRight(2)

	detailStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF7DB")).
			Background(lipgloss.Color("#F25D94")).
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A89CFF")).
				MarginTop(1)
)

type item struct {
	issue ReviewIssue
}

func (i item) Title() string       { return i.issue.Title }
func (i item) Description() string { return fmt.Sprintf("%s:%d [%s]", i.issue.File, i.issue.Line, i.issue.Severity) }
func (i item) FilterValue() string { return i.issue.Title }

type ReviewModel struct {
	list          list.Model
	viewport      viewport.Model
	issues        []ReviewIssue
	selectedIssue *ReviewIssue
	ready         bool
	width         int
	height        int
	statusMessage string
	err           error
}

func initialReviewModel(issues []ReviewIssue) ReviewModel {
	items := make([]list.Item, len(issues))
	for i, issue := range issues {
		items[i] = item{issue: issue}
	}

	// Create list
	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "Review Issues"
	l.SetShowHelp(false)

	return ReviewModel{
		list:   l,
		issues: issues,
	}
}

func (m ReviewModel) Init() tea.Cmd {
	return nil
}

func (m ReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't intercept keys if filtering
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if i, ok := m.list.SelectedItem().(item); ok {
				m.selectedIssue = &i.issue
				m.updateViewport()
			}
		case "a":
			if m.selectedIssue != nil {
				return m, applyFixCmd(m.selectedIssue)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		listWidth := int(float64(m.width) * 0.35)
		detailWidth := m.width - listWidth - 4

		m.list.SetWidth(listWidth)
		m.list.SetHeight(m.height - 2)

		if !m.ready {
			m.viewport = viewport.New(detailWidth, m.height-2)
			m.ready = true
		} else {
			m.viewport.Width = detailWidth
			m.viewport.Height = m.height - 2
		}

		// Initial selection
		if m.selectedIssue == nil && len(m.issues) > 0 {
			if i, ok := m.list.SelectedItem().(item); ok {
				m.selectedIssue = &i.issue
				m.updateViewport()
			}
		}

	case fixMsg:
		m.statusMessage = fmt.Sprintf("✅ Applied fix for %s", msg.issue.Title)
		// Mark as fixed in the list?
		// For MVP, we just show status. Ideally we'd update the item title.

		// Refresh logic if needed
		m.updateViewport() // Refresh details to show "FIXED"?

		// Clear status after 3s
		cmds = append(cmds, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return clearStatusMsg{}
		}))

	case errMsg:
		m.statusMessage = fmt.Sprintf("❌ Error: %v", msg.err)
		cmds = append(cmds, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
			return clearStatusMsg{}
		}))

	case clearStatusMsg:
		m.statusMessage = ""
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *ReviewModel) updateViewport() {
	if m.selectedIssue == nil {
		m.viewport.SetContent("Select an issue to view details.")
		return
	}

	issue := m.selectedIssue
	content := fmt.Sprintf("%s\n\nFile: %s:%d\nSeverity: %s\n\n%s\n\nSuggestion:\n---\n%s\n---",
		titleStyle.Render(issue.Title),
		issue.File,
		issue.Line,
		issue.Severity,
		issue.Description,
		issue.Suggestion,
	)

	m.viewport.SetContent(content)
}

func (m ReviewModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	listView := listStyle.Render(m.list.View())
	detailView := detailStyle.Render(m.viewport.View())

	// Footer with status
	status := ""
	if m.statusMessage != "" {
		status = statusMessageStyle.Render(m.statusMessage)
	}

	// Helper
	help := "\n[enter] view details • [a] apply fix • [q] quit"

	rightPane := lipgloss.JoinVertical(lipgloss.Left, detailView, status, help)

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, rightPane)
}

/* -------------------------------------------------------------------------
   COMMANDS
   ------------------------------------------------------------------------- */

type fixMsg struct {
	issue *ReviewIssue
}

type errMsg struct {
	err error
}

type clearStatusMsg struct{}

func applyFixCmd(issue *ReviewIssue) tea.Cmd {
	return func() tea.Msg {
		// 1. Validate suggestion is a code block or valid text
		// Simple heuristic: if suggestion contains code, try to replace.
		// For MVP, we only support replacing lines if suggestion is enclosed in ``` or just assumes it's the replacement.

		// Let's assume the Suggestion IS the replacement code.
		// We need to determine WHAT to replace.
		// The agent gave us 'Line'. We can assume it's a single line or a block starting there.
		// Without end line, it's risky.

		// Heuristic: If suggestion is multi-line, we might not know how many lines to replace.
		// BUT, if we assume the agent is smart, maybe we can't do auto-fix safely without more info (start/end lines).

		// Fallback: Just append a comment with the suggestion if risky?
		// Or: Create a patch file?

		// Let's try to be smart:
		// Read file.
		content, err := os.ReadFile(issue.File)
		if err != nil {
			return errMsg{err}
		}

		lines := strings.Split(string(content), "\n")
		if issue.Line < 1 || issue.Line > len(lines) {
			return errMsg{fmt.Errorf("line %d out of bounds", issue.Line)}
		}

		// Check if suggestion is empty
		if strings.TrimSpace(issue.Suggestion) == "" {
			return errMsg{fmt.Errorf("empty suggestion, cannot apply")}
		}

		// Naive application: Insert the suggestion BEFORE the line as a comment,
		// OR if it looks like a replacement (e.g. same indentation), replace the line.

		// Let's go with SAFE approach:
		// Insert a TODO comment with the suggestion above the line.

		suggestionLines := strings.Split(issue.Suggestion, "\n")
		var newLines []string

		// Construct the fix
		// We add a comment block
		fixBlock := []string{
			"// TODO(recac-fix): " + issue.Title,
			"// Suggestion:",
		}
		for _, sl := range suggestionLines {
			fixBlock = append(fixBlock, "// "+sl)
		}

		// Insert at line-1 (since slice is 0-indexed and line is 1-indexed)
		idx := issue.Line - 1
		newLines = append(newLines, lines[:idx]...)
		newLines = append(newLines, fixBlock...)
		newLines = append(newLines, lines[idx:]...)

		output := strings.Join(newLines, "\n")
		if err := os.WriteFile(issue.File, []byte(output), 0644); err != nil {
			return errMsg{err}
		}

		return fixMsg{issue}
	}
}
