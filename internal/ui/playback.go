package ui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LogEntry represents a structured log line from slog.
type LogEntry struct {
	Time    time.Time
	Level   string
	Msg     string
	Raw     map[string]interface{}
	Content string // Pretty printed content for details view
}

// Title returns the list item title.
func (e LogEntry) Title() string {
	return fmt.Sprintf("[%s] %s", e.Level, e.Msg)
}

// Description returns the list item description.
func (e LogEntry) Description() string {
	return e.Time.Format("15:04:05.000")
}

// FilterValue returns the value to filter by.
func (e LogEntry) FilterValue() string { return e.Level + " " + e.Msg }

// PlaybackModel is the TUI model for the playback command.
type PlaybackModel struct {
	list           list.Model
	viewport       viewport.Model
	viewingDetails bool
	entries        []LogEntry
	filtered       []LogEntry // For future advanced filtering if needed beyond list's built-in
	width          int
	height         int
}

// NewPlaybackModel creates a new playback model.
func NewPlaybackModel(entries []LogEntry) PlaybackModel {
	items := make([]list.Item, len(entries))
	for i, e := range entries {
		items[i] = e
	}

	delegate := list.NewDefaultDelegate()
	// Customize delegate styles if needed
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("205")).BorderLeftForeground(lipgloss.Color("205"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("205")).BorderLeftForeground(lipgloss.Color("205"))

	l := list.New(items, delegate, 0, 0)
	l.Title = "Session Playback"
	l.SetShowHelp(true)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
		}
	}

	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().Padding(1, 2)

	return PlaybackModel{
		list:     l,
		viewport: vp,
		entries:  entries,
	}
}

func (m PlaybackModel) Init() tea.Cmd {
	return nil
}

func (m PlaybackModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			case "esc", "q", "backspace":
				m.viewingDetails = false
				return m, nil
			default:
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			item := m.list.SelectedItem()
			if item != nil {
				entry := item.(LogEntry)
				m.viewingDetails = true
				m.viewport.SetContent(entry.Content)
				m.viewport.GotoTop()
			}
		}
	}

	if !m.viewingDetails {
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m PlaybackModel) View() string {
	if m.viewingDetails {
		return fmt.Sprintf("%s\n%s", m.headerView(), m.viewport.View())
	}
	return m.list.View()
}

func (m PlaybackModel) headerView() string {
	title := "Entry Details"
	line := strings.Repeat("─", max(0, m.viewport.Width-len(title)))
	return lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(title + line)
}

// ParseLogLines parses raw bytes from a JSONL file into LogEntries.
func ParseLogLines(data []byte) ([]LogEntry, error) {
	var entries []LogEntry
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var raw map[string]interface{}
		// Try to parse as JSON
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			// If not JSON, treat as raw text log
			// We synthesize a log entry for it
			entry := LogEntry{
				Time:    time.Now(), // Unknown time
				Level:   "TEXT",
				Msg:     line,
				Content: line,
				Raw:     map[string]interface{}{"msg": line},
			}
			entries = append(entries, entry)
			continue
		}

		// Extract standard fields
		entry := LogEntry{
			Raw: raw,
		}

		// Time
		if tStr, ok := raw["time"].(string); ok {
			if t, err := time.Parse(time.RFC3339, tStr); err == nil {
				entry.Time = t
			} else if t, err := time.Parse(time.RFC3339Nano, tStr); err == nil {
				entry.Time = t
			}
		}

		// Level
		if l, ok := raw["level"].(string); ok {
			entry.Level = l
		} else {
			entry.Level = "INFO"
		}

		// Msg
		if m, ok := raw["msg"].(string); ok {
			entry.Msg = m
		}

		// Build pretty content
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Time:  %s\n", entry.Time.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("Level: %s\n", entry.Level))
		sb.WriteString(fmt.Sprintf("Msg:   %s\n", entry.Msg))
		sb.WriteString("----------------------------------------\n")

		// Sort keys for deterministic output
		var keys []string
		for k := range raw {
			if k != "time" && k != "level" && k != "msg" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := raw[k]
			// Pretty print complex values
			valStr := fmt.Sprintf("%v", v)
			if m, ok := v.(map[string]interface{}); ok {
				if b, err := json.MarshalIndent(m, "", "  "); err == nil {
					valStr = string(b)
				}
			} else if arr, ok := v.([]interface{}); ok {
				if b, err := json.MarshalIndent(arr, "", "  "); err == nil {
					valStr = string(b)
				}
			}

			sb.WriteString(fmt.Sprintf("\n[%s]:\n%s\n", k, valStr))
		}

		entry.Content = sb.String()
		entries = append(entries, entry)
	}

	return entries, nil
}
