package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- TUI Model ---

type TUIModel struct {
	ActiveAgents map[string]WorkItem
	AgentStatus  map[string]string // "Spawning", "Running", "Failed"
	PollStatus   string
	LastPoll     time.Time
	Logs         []string
	Err          error
	width        int
	height       int
}

func InitialModel() *TUIModel {
	return &TUIModel{
		ActiveAgents: make(map[string]WorkItem),
		AgentStatus:  make(map[string]string),
		Logs:         make([]string, 0, 50),
		PollStatus:   "Waiting...",
	}
}

func (m *TUIModel) Init() tea.Cmd {
	return nil
}

func (m *TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case MsgPollStart:
		m.PollStatus = "Polling..."
	case MsgPollEnd:
		if msg.Err != nil {
			m.PollStatus = fmt.Sprintf("Poll Failed: %v", msg.Err)
		} else {
			m.PollStatus = fmt.Sprintf("Poll OK (%d items) at %s", msg.Count, time.Now().Format("15:04:05"))
		}
		m.LastPoll = time.Now()
	case MsgSpawnStart:
		m.ActiveAgents[msg.Item.ID] = msg.Item
		m.AgentStatus[msg.Item.ID] = "Spawning"
	case MsgSpawnEnd:
		if msg.Err != nil {
			m.AgentStatus[msg.Item.ID] = fmt.Sprintf("Failed: %v", msg.Err)
		} else {
			m.AgentStatus[msg.Item.ID] = "Running"
		}
	case MsgLog:
		m.Logs = append(m.Logs, msg.Content)
		if len(m.Logs) > 20 { // Keep last 20 lines
			m.Logs = m.Logs[1:]
		}
	}
	return m, nil
}

func (m *TUIModel) View() string {
	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	logStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#505050"))

	s := titleStyle.Render("Recac Orchestrator") + "\n\n"
	s += fmt.Sprintf("Status: %s\n", m.PollStatus)
	s += fmt.Sprintf("Active Agents: %d\n", len(m.ActiveAgents))
	s += "\nAgents:\n"

	// Sort IDs for stable display
	ids := make([]string, 0, len(m.AgentStatus))
	for id := range m.AgentStatus {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		status := m.AgentStatus[id]
		item := m.ActiveAgents[id]
		statusColor := "#00FF00" // Green
		if status == "Spawning" {
			statusColor = "#FFFF00" // Yellow
		} else if status != "Running" {
			statusColor = "#FF0000" // Red
		}
		itemStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor))
		s += fmt.Sprintf("- %s: %s [%s]\n", id, itemStyle.Render(status), item.Summary)
	}

	s += "\nLogs:\n"
	for _, l := range m.Logs {
		s += logStyle.Render(l) + "\n"
	}

	return s
}

// --- Messages ---

type MsgPollStart struct{}
type MsgPollEnd struct {
	Count int
	Err   error
}
type MsgSpawnStart struct {
	Item WorkItem
}
type MsgSpawnEnd struct {
	Item WorkItem
	Err  error
}
type MsgLog struct {
	Content string
}

// --- Observer ---

type TUIObserver struct {
	Program *tea.Program
}

func NewTUIObserver(p *tea.Program) *TUIObserver {
	return &TUIObserver{Program: p}
}

func (o *TUIObserver) OnPollStart() {
	o.Program.Send(MsgPollStart{})
}

func (o *TUIObserver) OnPollEnd(count int, err error) {
	o.Program.Send(MsgPollEnd{Count: count, Err: err})
}

func (o *TUIObserver) OnSpawnStart(item WorkItem) {
	o.Program.Send(MsgSpawnStart{Item: item})
}

func (o *TUIObserver) OnSpawnEnd(item WorkItem, err error) {
	o.Program.Send(MsgSpawnEnd{Item: item, Err: err})
}

// --- Log Handler ---

type TUILogHandler struct {
	Program *tea.Program
	Wrapped slog.Handler
}

func NewTUILogHandler(p *tea.Program, wrapped slog.Handler) *TUILogHandler {
	return &TUILogHandler{Program: p, Wrapped: wrapped}
}

func (h *TUILogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Wrapped.Enabled(ctx, level)
}

func (h *TUILogHandler) Handle(ctx context.Context, r slog.Record) error {
	msg := fmt.Sprintf("[%s] %s", r.Level.String(), r.Message)
	// Send to TUI
	h.Program.Send(MsgLog{Content: msg})
	// Also log to underlying handler
	return h.Wrapped.Handle(ctx, r)
}

func (h *TUILogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TUILogHandler{Program: h.Program, Wrapped: h.Wrapped.WithAttrs(attrs)}
}

func (h *TUILogHandler) WithGroup(name string) slog.Handler {
	return &TUILogHandler{Program: h.Program, Wrapped: h.Wrapped.WithGroup(name)}
}
