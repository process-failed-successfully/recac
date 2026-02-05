package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CommitItem represents a git commit in the list.
type CommitItem struct {
	Hash    string
	Author  string
	Date    string
	Message string
}

func (i CommitItem) Title() string {
	return fmt.Sprintf("%s - %s", i.Hash, i.Message)
}

func (i CommitItem) Description() string {
	return fmt.Sprintf("%s | %s", i.Author, i.Date)
}

func (i CommitItem) FilterValue() string { return i.Message + " " + i.Hash + " " + i.Author }

// GitLogModel is the Bubble Tea model for the git log explorer.
type GitLogModel struct {
	list           list.Model
	viewport       viewport.Model
	viewingDetails bool   // If true, showing viewport (diff/explanation)
	statusMessage  string // For temporary status like "Analyzing..."

	// Callbacks
	fetchDiffFunc func(hash string) (string, error)
	explainFunc   AnalysisFunc
	auditFunc     AnalysisFunc

	width  int
	height int
}

// NewGitLogModel creates a new git log model.
func NewGitLogModel(commits []CommitItem, fetchDiff func(string) (string, error), explain, audit AnalysisFunc) GitLogModel {
	m := GitLogModel{
		fetchDiffFunc: fetchDiff,
		explainFunc:   explain,
		auditFunc:     audit,
	}

	// Initialize List
	items := make([]list.Item, len(commits))
	for i, c := range commits {
		items[i] = c
	}

	delegate := list.NewDefaultDelegate()
	m.list = list.New(items, delegate, 0, 0)
	m.list.Title = "Git Log"
	m.list.SetShowHelp(true)
	m.list.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "view diff")),
		}
	}
	m.list.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "view diff")),
			key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "explain (AI)")),
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "security audit (AI)")),
		}
	}

	// Initialize Viewport
	m.viewport = viewport.New(0, 0)

	return m
}

type diffMsg struct {
	content string
	err     error
}

type analysisResultMsg struct {
	result string
	err    error
}

func (m GitLogModel) Init() tea.Cmd {
	return nil
}

func (m GitLogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 1
		contentHeight := msg.Height - headerHeight
		if contentHeight < 0 {
			contentHeight = 0
		}

		m.list.SetSize(msg.Width, contentHeight)
		m.viewport.Width = msg.Width
		m.viewport.Height = contentHeight
		return m, nil

	case tea.KeyMsg:
		if m.viewingDetails {
			switch msg.String() {
			case "q", "esc", "backspace":
				m.viewingDetails = false
				m.statusMessage = ""
				return m, nil
			default:
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		}

		// List Mode
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			item := m.list.SelectedItem()
			if item != nil && m.fetchDiffFunc != nil {
				commit := item.(CommitItem)
				m.statusMessage = "Fetching diff..."
				return m, func() tea.Msg {
					diff, err := m.fetchDiffFunc(commit.Hash)
					return diffMsg{content: diff, err: err}
				}
			}

		case "e": // Explain
			item := m.list.SelectedItem()
			if item != nil && m.explainFunc != nil {
				commit := item.(CommitItem)
				m.statusMessage = "🤖 Asking AI to explain commit..."
				return m, m.runAnalysis(m.explainFunc, commit.Hash)
			}

		case "s": // Security Audit
			item := m.list.SelectedItem()
			if item != nil && m.auditFunc != nil {
				commit := item.(CommitItem)
				m.statusMessage = "🔍 Auditing commit for security..."
				return m, m.runAnalysis(m.auditFunc, commit.Hash)
			}
		}

	case diffMsg:
		m.statusMessage = ""
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Error fetching diff: %v", msg.err)
		} else {
			m.viewingDetails = true
			m.viewport.SetContent(msg.content)
			m.viewport.GotoTop()
		}

	case analysisResultMsg:
		m.statusMessage = ""
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.viewingDetails = true
			m.viewport.SetContent(msg.result)
			m.viewport.GotoTop()
		}
	}

	if !m.viewingDetails {
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m GitLogModel) runAnalysis(fn AnalysisFunc, hash string) tea.Cmd {
	return func() tea.Msg {
		res, err := fn(hash)
		return analysisResultMsg{result: res, err: err}
	}
}

func (m GitLogModel) View() string {
	if m.viewingDetails {
		return fmt.Sprintf("%s\n%s", m.headerView(), m.viewport.View())
	}
	return fmt.Sprintf("%s\n%s", m.statusView(), m.list.View())
}

func (m GitLogModel) headerView() string {
	title := "Commit Details"
	line := strings.Repeat("─", max(0, m.viewport.Width-len(title)))
	return lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(title + line)
}

func (m GitLogModel) statusView() string {
	if m.statusMessage == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(m.statusMessage)
}
