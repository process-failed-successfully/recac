package tui

import (
	"context"
	"fmt"
	"os"
	"recac/internal/agent"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type SessionModel struct {
	agent           agent.Agent
	viewport        viewport.Model
	input           textarea.Model
	renderer        *glamour.TermRenderer
	err             error
	isLoading       bool
	history         string // Raw history of completed turns + current user message
	renderedHistory string // Rendered history of completed turns + current user message
	currentResponse string // Raw text of current streaming response
	messages        []Message
	ctx             context.Context
	contextFiles    map[string]string // path -> content
}

type Message struct {
	Role    string
	Content string
}

type errMsg error
type chunkMsg struct {
	chunk string
	ch    <-chan string
}
type doneMsg struct{}

func NewSessionModel(ag agent.Agent) SessionModel {
	ta := textarea.New()
	ta.Placeholder = "Type your message (Enter to send)..."
	ta.Focus()
	ta.CharLimit = 0
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	initialText := "Welcome to RECAC Interactive Session!\nType /help for commands.\n\n"
	vp := viewport.New(0, 0)
	vp.SetContent(initialText)

	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	return SessionModel{
		agent:           ag,
		messages:        []Message{},
		viewport:        vp,
		input:           ta,
		renderer:        r,
		ctx:             context.Background(),
		history:         "",
		renderedHistory: initialText,
		currentResponse: "",
		contextFiles:    make(map[string]string),
	}
}

func StartSession(ag agent.Agent) error {
	p := tea.NewProgram(NewSessionModel(ag), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func (m SessionModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m SessionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.input, tiCmd = m.input.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			if !m.isLoading {
				v := m.input.Value()
				if v == "" {
					return m, nil
				}

				// Handle commands
				if strings.HasPrefix(v, "/") {
					return m.handleCommand(v)
				}

				// User message
				m.input.Reset()
				m.isLoading = true

				// Add to history and render
				userMsg := fmt.Sprintf("**User**: %s\n\n", v)
				m.appendHistory(userMsg)

				// Start streaming
				return m, tea.Batch(tiCmd, m.sendRequest(v))
			}
		case "ctrl+l":
			// Clear screen shortcut
			m.history = ""
			m.renderedHistory = ""
			m.currentResponse = ""
			m.viewport.SetContent("")
			return m, nil
		}

	case chunkMsg:
		m.currentResponse += msg.chunk
		// Update viewport with rendered history + raw current response
		m.viewport.SetContent(m.renderedHistory + m.currentResponse)
		m.viewport.GotoBottom()
		return m, waitForChunk(msg.ch)

	case doneMsg:
		m.isLoading = false
		// Commit current response to history
		m.history += m.currentResponse + "\n\n"

		// Render the full response properly
		renderedResp := m.render(m.currentResponse + "\n\n")
		m.renderedHistory += renderedResp
		m.currentResponse = ""

		m.viewport.SetContent(m.renderedHistory)
		m.viewport.GotoBottom()
		return m, nil

	case errMsg:
		m.err = msg
		m.isLoading = false
		errorMsg := fmt.Sprintf("\n**Error**: %v\n\n", msg)
		m.appendHistory(errorMsg)
		return m, nil

	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - m.input.Height() - 4 // margin
		m.input.SetWidth(msg.Width)
		// Re-render everything if size changed (to fix word wrap)
		// Ideally we re-render m.history entirely
		if m.renderer != nil {
			// Update renderer with new width?
			// glamour renderer is immutable-ish, need new one?
			// Actually WithWordWrap is set on creation.
			// Creating new renderer is cheap.
			r, _ := glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(msg.Width),
			)
			m.renderer = r
			// Re-render full history
			m.renderedHistory = m.render(m.history)
			m.viewport.SetContent(m.renderedHistory + m.currentResponse)
		}
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *SessionModel) appendHistory(text string) {
	m.history += text
	m.renderedHistory += m.render(text)
	m.viewport.SetContent(m.renderedHistory)
	m.viewport.GotoBottom()
}

func (m SessionModel) render(text string) string {
	if m.renderer != nil {
		rendered, err := m.renderer.Render(text)
		if err == nil {
			return rendered
		}
	}
	return text
}

func (m SessionModel) handleCommand(input string) (tea.Model, tea.Cmd) {
	m.input.Reset()
	parts := strings.Fields(input)
	cmd := parts[0]

	switch cmd {
	case "/quit", "/exit":
		return m, tea.Quit
	case "/clear":
		m.history = ""
		m.renderedHistory = ""
		m.currentResponse = ""
		m.viewport.SetContent("History cleared.\n\n")
		return m, nil
	case "/add":
		if len(parts) < 2 {
			m.appendHistory("Usage: /add <file_path>\n")
			return m, nil
		}
		// Handle paths with spaces
		path := strings.TrimSpace(strings.TrimPrefix(input, "/add"))
		content, err := os.ReadFile(path)
		if err != nil {
			m.appendHistory(fmt.Sprintf("Failed to read file: %v\n", err))
			return m, nil
		}
		m.contextFiles[path] = string(content)
		m.appendHistory(fmt.Sprintf("Added %s to context (%d bytes).\n", path, len(content)))
		return m, nil
	case "/context":
		if len(m.contextFiles) == 0 {
			m.appendHistory("No files in context.\n")
		} else {
			m.appendHistory("Current context files:\n")
			// Sort keys for deterministic output
			keys := make([]string, 0, len(m.contextFiles))
			for k := range m.contextFiles {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, path := range keys {
				content := m.contextFiles[path]
				m.appendHistory(fmt.Sprintf("- %s (%d bytes)\n", path, len(content)))
			}
		}
		return m, nil
	case "/help":
		help := `
**Available commands:**
- /add <file>: Add file content to context
- /context: List current context files
- /quit, /exit: Quit the session
- /clear: Clear chat history
- /help: Show this help
`
		m.appendHistory(help)
		return m, nil
	}

	m.appendHistory(fmt.Sprintf("Unknown command: %s\n", cmd))
	return m, nil
}

func (m SessionModel) sendRequest(prompt string) tea.Cmd {
	ch := make(chan string, 100)

	// Create a copy of the prompt to avoid race conditions if history changes (it shouldn't)
	// Build prompt uses m.history which includes the user message we just added.
	fullPrompt := m.buildPrompt()

	go func() {
		defer close(ch)
		_, err := m.agent.SendStream(m.ctx, fullPrompt, func(chunk string) {
			ch <- chunk
		})
		if err != nil {
			// In a real app we'd handle error better
		}
	}()

	return waitForChunk(ch)
}

func (m SessionModel) buildPrompt() string {
	var sb strings.Builder

	// 1. Context Files
	if len(m.contextFiles) > 0 {
		sb.WriteString("Context Files:\n")
		// Sort keys for deterministic prompt generation
		keys := make([]string, 0, len(m.contextFiles))
		for k := range m.contextFiles {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, path := range keys {
			content := m.contextFiles[path]
			sb.WriteString(fmt.Sprintf("--- %s ---\n%s\n--- End of %s ---\n\n", path, content, path))
		}
	}

	// 2. Chat History (includes current message)
	sb.WriteString(m.history)
	sb.WriteString("Assistant:")

	return sb.String()
}

func waitForChunk(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		chunk, ok := <-ch
		if !ok {
			return doneMsg{}
		}
		return chunkMsg{chunk: chunk, ch: ch}
	}
}

func (m SessionModel) View() string {
	return fmt.Sprintf(
		"%s\n%s",
		m.viewport.View(),
		lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true, false, false, false).Render(m.input.View()),
	)
}
