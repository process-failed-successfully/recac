package tui

import (
	"context"
	"fmt"
	"os"
	"recac/internal/agent"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
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
	spinner         spinner.Model
	renderer        *glamour.TermRenderer
	err             error
	isLoading       bool
	history         string // Raw history of completed turns + current user message
	renderedHistory string // Rendered history of completed turns + current user message
	currentResponse string // Raw text of current streaming response
	messages        []Message
	ctx             context.Context
	contextFiles    map[string]string // path -> content

	// Persona support
	persona        agent.Persona
	personaManager *agent.PersonaManager
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

func NewSessionModel(ag agent.Agent, initialPersonaID string) SessionModel {
	ta := textarea.New()
	ta.Placeholder = "Type your message (Enter to send)..."
	ta.Focus()
	ta.CharLimit = 0
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	initialText := "Welcome to RECAC Interactive Session!\nType /help for commands.\n\n"
	vp := viewport.New(0, 0)

	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Initialize Persona Manager
	pm := agent.NewPersonaManager()
	if err := pm.LoadPersonas(); err != nil {
		initialText += fmt.Sprintf("Warning: Failed to load personas: %v\n\n", err)
	}

	// Set initial persona
	persona, ok := pm.GetPersona(initialPersonaID)
	if !ok {
		// Fallback
		persona, _ = pm.GetPersona("default")
	}

	// Prepend persona info to history
	personaMsg := fmt.Sprintf("**System**: Persona set to **%s**\n_%s_\n\n", persona.Name, persona.Description)
	initialText += personaMsg
	renderedInitial, _ := r.Render(initialText)
	vp.SetContent(renderedInitial)

	return SessionModel{
		agent:           ag,
		messages:        []Message{},
		viewport:        vp,
		input:           ta,
		spinner:         s,
		renderer:        r,
		ctx:             context.Background(),
		history:         initialText,
		renderedHistory: renderedInitial,
		currentResponse: "",
		contextFiles:    make(map[string]string),
		persona:         persona,
		personaManager:  pm,
	}
}

func StartSession(ag agent.Agent, initialPersonaID string) error {
	p := tea.NewProgram(NewSessionModel(ag, initialPersonaID), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func (m SessionModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick)
}

func (m SessionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd   tea.Cmd
		vpCmd   tea.Cmd
		spinCmd tea.Cmd
	)

	m.input, tiCmd = m.input.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	m.spinner, spinCmd = m.spinner.Update(msg)

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
				m.input.Blur()

				// Add to history and render
				userMsg := fmt.Sprintf("**User**: %s\n\n", v)
				m.appendHistory(userMsg)

				// Start streaming
				return m, tea.Batch(tiCmd, m.sendRequest(v), spinCmd)
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
		return m, tea.Batch(waitForChunk(msg.ch), spinCmd)

	case doneMsg:
		m.isLoading = false
		m.input.Focus()
		// Commit current response to history
		m.history += m.currentResponse + "\n\n"

		// Render the full response properly
		renderedResp := m.render(m.currentResponse + "\n\n")
		m.renderedHistory += renderedResp
		m.currentResponse = ""

		m.viewport.SetContent(m.renderedHistory)
		m.viewport.GotoBottom()
		return m, spinCmd

	case errMsg:
		m.err = msg
		m.isLoading = false
		m.input.Focus()
		errorMsg := fmt.Sprintf("\n**Error**: %v\n\n", msg)
		m.appendHistory(errorMsg)
		return m, spinCmd

	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - m.input.Height() - 6 // margin
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

	return m, tea.Batch(tiCmd, vpCmd, spinCmd)
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
	case "/persona":
		if len(parts) < 2 {
			m.appendHistory("Usage: /persona <name>\nAvailable personas:\n")
			for _, k := range m.personaManager.ListSorted() {
				m.appendHistory(fmt.Sprintf("- %s\n", k))
			}
			return m, nil
		}
		name := parts[1]
		if p, ok := m.personaManager.GetPersona(name); ok {
			m.persona = p
			m.appendHistory(fmt.Sprintf("**System**: Switched persona to **%s**\n_%s_\n\n", p.Name, p.Description))
		} else {
			m.appendHistory(fmt.Sprintf("Unknown persona '%s'.\n", name))
		}
		return m, nil
	case "/help":
		help := `
**Available commands:**
- /add <file>: Add file content to context
- /context: List current context files
- /persona <name>: Switch AI persona
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

	// 1. System Prompt (Persona)
	sb.WriteString(m.persona.SystemPrompt)
	sb.WriteString("\n\n")

	// 2. Context Files
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

	// 3. Chat History (includes current message)
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
	borderColor := lipgloss.Color("63") // Default purple
	if m.isLoading {
		borderColor = lipgloss.Color("240") // Dimmed grey
	}

	inputView := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(borderColor).
		Render(m.input.View())

	footer := inputView
	if m.isLoading {
		footer = fmt.Sprintf("%s Thinking...\n%s", m.spinner.View(), inputView)
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	helpBar := helpStyle.Render(fmt.Sprintf("Persona: %s • Esc/Ctrl+C: quit • Enter: send • /help commands", m.persona.Name))

	return fmt.Sprintf(
		"%s\n%s\n%s",
		m.viewport.View(),
		footer,
		helpBar,
	)
}
