package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// -- Styling --

var (
	interactiveAppStyle = lipgloss.NewStyle().Margin(0, 0) // Full bleed mostly

	interactiveTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFF")).
				Background(lipgloss.Color("#874BFD")).
				Padding(0, 1)

	interactiveStatusMessageStyle = lipgloss.NewStyle().
					Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"})

	interactiveSenderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))      // Blue for User
	interactiveBotStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))     // Pink for Recac
	interactiveShellStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4B400")) // Yellow for Shell

	// Layout styles
	interactiveHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("228")).
				MarginLeft(2).
				Bold(true)

	interactiveListStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder(), false, false, false, false). // Simplified border
				BorderForeground(lipgloss.Color("63")).
				MarginRight(2)

	promptStyle = lipgloss.NewStyle().MarginLeft(2)

	infoBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")). // Dim gray
			MarginLeft(2).
			MarginBottom(1)
)

// -- Key Bindings --

type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	Enter      key.Binding
	Slash      key.Binding
	Bang       key.Binding // '!' for shell
	Quit       key.Binding
	ToggleList key.Binding
	Back       key.Binding // Esc to go back from menus
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Slash, k.Bang, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter},
		{k.Slash, k.Bang, k.ToggleList, k.Quit},
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "send"),
	),
	Slash: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "cmds"),
	),
	Bang: key.NewBinding(
		key.WithKeys("!"),
		key.WithHelp("!", "shell"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	),
	ToggleList: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "toggle cmds"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
}

// -- Data Models --

type InputMode int

const (
	ModeChat InputMode = iota
	ModeCmd
	ModeShell
	ModeModelSelect // Model menu
	ModeAgentSelect // Agent/Provider menu
)

// CommandItem implements list.Item
type CommandItem struct {
	Name   string
	Desc   string
	Action func(m *InteractiveModel, args []string) tea.Cmd
}

func (i CommandItem) FilterValue() string { return i.Name }
func (i CommandItem) Title() string       { return i.Name }
func (i CommandItem) Description() string { return i.Desc }

// ModelItem implements list.Item for the model menu
type ModelItem struct {
	Name               string
	Value              string
	DescriptionDetails string
}

func (i ModelItem) FilterValue() string { return i.Name }
func (i ModelItem) Title() string       { return i.Name }
func (i ModelItem) Description() string { return i.DescriptionDetails }

// AgentItem implements list.Item for the agent/provider menu
type AgentItem struct {
	Name               string
	Value              string // Provider ID
	DescriptionDetails string
}

func (i AgentItem) FilterValue() string { return i.Name }
func (i AgentItem) Title() string       { return i.Name }
func (i AgentItem) Description() string { return i.DescriptionDetails }

// SlashCommand legacy wrapper
type SlashCommand struct {
	Name        string
	Description string
	Action      func(m *InteractiveModel, args []string) tea.Cmd
}

type InteractiveModel struct {
	viewport viewport.Model
	textarea textarea.Model

	list    list.Model
	help    help.Model
	spinner spinner.Model
	keys    keyMap

	history  []string
	commands []CommandItem

	// Data
	agents      []AgentItem            // Available agents/providers
	agentModels map[string][]ModelItem // Models keyed by agent value

	currentModel string // Selected model ID
	currentAgent string // Selected agent/provider

	mode     InputMode
	showList bool
	thinking bool // For spinner

	err error

	// Layout
	width  int
	height int
}

func NewInteractiveModel(commands []SlashCommand) InteractiveModel {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 1000
	ta.SetWidth(50)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	// Convert SlashCommands to CommandItems
	// Custom Commands Injection
	items := make([]list.Item, 0)
	cmdItems := make([]CommandItem, 0)

	hasModelCmd := false
	hasAgentCmd := false
	for _, c := range commands {
		if c.Name == "/model" {
			hasModelCmd = true
		}
		if c.Name == "/agent" {
			hasAgentCmd = true
		}
	}

	// Add built-in /model command
	if !hasModelCmd {
		modelCmd := CommandItem{
			Name: "/model",
			Desc: "Select active AI model from current Agent",
			Action: func(m *InteractiveModel, args []string) tea.Cmd {
				m.setMode(ModeModelSelect)
				return nil
			},
		}
		items = append(items, modelCmd)
		cmdItems = append(cmdItems, modelCmd)
	}

	// Add built-in /agent command
	if !hasAgentCmd {
		agentCmd := CommandItem{
			Name: "/agent",
			Desc: "Select active Agent Provider (OpenAI, Gemini, etc)",
			Action: func(m *InteractiveModel, args []string) tea.Cmd {
				m.setMode(ModeAgentSelect)
				return nil
			},
		}
		items = append(items, agentCmd)
		cmdItems = append(cmdItems, agentCmd)
	}

	for _, c := range commands {
		item := CommandItem{
			Name:   c.Name,
			Desc:   c.Description,
			Action: c.Action,
		}
		items = append(items, item)
		cmdItems = append(cmdItems, item)
	}

	// Setup List
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Slash Commands"
	l.SetShowHelp(false)
	l.SetHeight(6)
	l.DisableQuitKeybindings()
	l.Styles.Title = interactiveTitleStyle

	// Spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	vp := viewport.New(50, 10)
	welcomeMsg := strings.Join([]string{
		interactiveBotStyle.Render("Recac: ") + "Welcome to RECAC! 🎨",
		"",
		interactiveStatusMessageStyle.Render("  • Type / for commands (or press Tab)"),
		interactiveStatusMessageStyle.Render("  • Type ! for shell execution"),
		interactiveStatusMessageStyle.Render("  • Type anything else to chat"),
		interactiveStatusMessageStyle.Render("  • Press Ctrl+C to quit"),
		"",
	}, "\n")
	vp.SetContent(welcomeMsg)

	// Define Agents/Providers
	availableAgents := []AgentItem{
		{Name: "Gemini", Value: "gemini", DescriptionDetails: "Google DeepMind Gemini Models"},
		{Name: "OpenAI", Value: "openai", DescriptionDetails: "OpenAI GPT Models"},
		{Name: "OpenRouter", Value: "openrouter", DescriptionDetails: "Models via OpenRouter"},
		{Name: "Ollama", Value: "ollama", DescriptionDetails: "Local Models via Ollama"},
		{Name: "Anthropic", Value: "anthropic", DescriptionDetails: "Anthropic Claude Models"},
		{Name: "Cursor CLI", Value: "cursor-cli", DescriptionDetails: "Cursor Editor CLI Integration"},
		{Name: "Gemini CLI", Value: "gemini-cli", DescriptionDetails: "Google Gemini CLI Integration"},
	}

	// Define Models per Agent
	agentModels := make(map[string][]ModelItem)

	agentModels["openai"] = []ModelItem{
		{Name: "GPT-4o", Value: "gpt-4o", DescriptionDetails: "Omni model, high intelligence"},
		{Name: "GPT-4 Turbo", Value: "gpt-4-turbo", DescriptionDetails: "High intelligence"},
		{Name: "GPT-3.5 Turbo", Value: "gpt-3.5-turbo", DescriptionDetails: "Fast and cheap"},
	}

	// Try to load OpenRouter models from file
	if orModels, err := loadModelsFromFile("openrouter-models.json"); err == nil && len(orModels) > 0 {
		agentModels["openrouter"] = orModels
	} else {
		// Fallback
		agentModels["openrouter"] = []ModelItem{
			{Name: "Anthropic Claude 3.5 Sonnet", Value: "anthropic/claude-3.5-sonnet", DescriptionDetails: "High intelligence"},
			{Name: "Google Gemini Pro 1.5", Value: "google/gemini-pro-1.5", DescriptionDetails: "Long context"},
			{Name: "Meta Llama 3 70B", Value: "meta-llama/llama-3-70b-instruct", DescriptionDetails: "Open source"},
		}
	}

	// Try to load Gemini models from file
	if geminiModels, err := loadModelsFromFile("gemini-models.json"); err == nil && len(geminiModels) > 0 {
		agentModels["gemini"] = geminiModels
	} else {
		agentModels["gemini"] = []ModelItem{
			{Name: "Gemini 2.0 Flash (Auto)", Value: "gemini-2.0-flash-auto", DescriptionDetails: "Best for most tasks"},
			{Name: "Gemini 2.0 Pro", Value: "gemini-2.0-pro", DescriptionDetails: "High reasoning capability"},
			{Name: "Gemini 2.0 Flash", Value: "gemini-2.0-flash", DescriptionDetails: "Fastest response time"},
			{Name: "Gemini 2.0 Flash Exp", Value: "gemini-2.0-flash-exp", DescriptionDetails: "Experimental features"},
			{Name: "Gemini 2.5 Flash", Value: "gemini-2.5-flash", DescriptionDetails: "Mid-size multimodal model"},
			{Name: "Gemini 2.5 Pro", Value: "gemini-2.5-pro", DescriptionDetails: "Stable release (June 2025)"},
			{Name: "Gemini 1.5 Pro", Value: "gemini-1.5-pro", DescriptionDetails: "Legacy stable model"},
		}
	}

	agentModels["ollama"] = []ModelItem{
		{Name: "Llama 3", Value: "llama3", DescriptionDetails: "Meta's Llama 3"},
		{Name: "Mistral", Value: "mistral", DescriptionDetails: "Mistral AI"},
		{Name: "Gemma 2", Value: "gemma2", DescriptionDetails: "Google's Gemma"},
		{Name: "Codellama", Value: "codellama", DescriptionDetails: "Code specialized"},
	}

	agentModels["anthropic"] = []ModelItem{
		{Name: "Claude 3.5 Sonnet", Value: "claude-3-5-sonnet-20240620", DescriptionDetails: "Balanced"},
		{Name: "Claude 3 Opus", Value: "claude-3-opus-20240229", DescriptionDetails: "Most powerful"},
		{Name: "Claude 3 Haiku", Value: "claude-3-haiku-20240307", DescriptionDetails: "Fastest"},
	}

	agentModels["cursor-cli"] = []ModelItem{
		{Name: "Auto", Value: "auto", DescriptionDetails: "Cursor Default"},
		{Name: "Claude 3.5 Sonnet", Value: "claude-3.5-sonnet", DescriptionDetails: "Specific Model"},
		{Name: "GPT-4o", Value: "gpt-4o", DescriptionDetails: "OpenAI via Cursor"},
	}

	agentModels["gemini-cli"] = []ModelItem{
		{Name: "Auto", Value: "auto", DescriptionDetails: "Gemini CLI Auto Selection"},
		{Name: "Pro", Value: "pro", DescriptionDetails: "Gemini 1.5 Pro"},
	}

	return InteractiveModel{
		textarea:     ta,
		viewport:     vp,
		list:         l,
		help:         help.New(),
		spinner:      s,
		keys:         keys,
		commands:     cmdItems,
		agents:       availableAgents,
		agentModels:  agentModels,
		currentModel: "gemini-2.0-flash-auto",
		currentAgent: "gemini",
		history:      []string{welcomeMsg},
		mode:         ModeChat,
		showList:     false,
		thinking:     false,
	}
}

func (m InteractiveModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick)
}

func (m InteractiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd   tea.Cmd
		vpCmd   tea.Cmd
		listCmd tea.Cmd
		spinCmd tea.Cmd
	)

	m.spinner, spinCmd = m.spinner.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	// Mode-based Updates
	switch m.mode {
	case ModeModelSelect, ModeAgentSelect:
		// In menu modes, list handles input
		m.list.SetHeight(12)
		m.list.SetShowTitle(true)
		m.list, listCmd = m.list.Update(msg)

	default:
		// Chat/Cmd/Shell: Textarea handles input
		m.textarea, tiCmd = m.textarea.Update(msg)

		// Manual Filtering Logic
		if m.showList {
			val := m.textarea.Value()
			if strings.HasPrefix(val, "/") {
				// Filter Logic
				query := strings.TrimPrefix(val, "/")
				if query == "" {
					// No query, show all
					m.setListItemsToCommands() // Or current context items?
				} else {
					// Filter commands
					var filtered []list.Item
					for _, c := range m.commands {
						if strings.Contains(strings.ToLower(c.Name), strings.ToLower(query)) { // Match /name against /query
							filtered = append(filtered, c)
						}
						// Also match description?
					}
					m.list.SetItems(filtered)
				}

				// Forward navigation keys to list even if textarea focuses
				switch msg := msg.(type) {
				case tea.KeyMsg:
					switch msg.Type {
					case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
						m.list, listCmd = m.list.Update(msg)
					}
				}
			} else {
				// Not a command, possibly clear list or hide it logic is elsewhere
			}
		}
	}

	switch msg := msg.(type) {
	case shellOutputMsg:
		m.conversation(string(msg), false)
		m.thinking = false
		m.setMode(ModeChat) // Return to chat after command
		return m, nil

	case StatusMsg:
		m.conversation(string(msg), false)
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 6
		footerHeight := 4
		inputHeight := m.textarea.Height() + 2

		vpHeight := msg.Height - headerHeight - footerHeight - inputHeight
		if vpHeight < 5 {
			vpHeight = 5
		}

		m.viewport.Width = msg.Width - 4
		m.viewport.Height = vpHeight

		m.textarea.SetWidth(msg.Width - 4)
		m.list.SetWidth(msg.Width - 4)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Back):
			if m.mode == ModeModelSelect || m.mode == ModeAgentSelect {
				m.setMode(ModeChat) // Cancel menu
				return m, nil
			}
			if m.showList {
				m.showList = false
				m.setMode(ModeChat)
				return m, nil
			}

		case key.Matches(msg, m.keys.ToggleList):
			if m.mode != ModeModelSelect && m.mode != ModeAgentSelect {
				m.toggleList()
				return m, nil
			}

		case key.Matches(msg, m.keys.Enter):
			// 1. Model Selection Mode
			if m.mode == ModeModelSelect {
				if i := m.list.SelectedItem(); i != nil {
					model := i.(ModelItem)
					m.currentModel = model.Value
					m.conversation(fmt.Sprintf("Switched model to: %s", model.Name), false)
					m.setMode(ModeChat)
				}
				return m, nil
			}

			// 2. Agent Selection Mode
			if m.mode == ModeAgentSelect {
				if i := m.list.SelectedItem(); i != nil {
					agent := i.(AgentItem)
					m.currentAgent = agent.Value
					m.conversation(fmt.Sprintf("Switched Agent Provider to: %s", agent.Name), false)

					// Update default model
					if models, ok := m.agentModels[m.currentAgent]; ok && len(models) > 0 {
						m.currentModel = models[0].Value
						m.conversation(fmt.Sprintf("Default model: %s", models[0].Name), false)
					}

					m.setMode(ModeChat)
				}
				return m, nil
			}

			// 3. PRIORITY: Check Manual Input Match First
			v := m.textarea.Value()
			if v != "" && strings.HasPrefix(v, "/") {
				parts := strings.Fields(v)
				if len(parts) > 0 {
					cmdName := parts[0]

					// Built-in checks
					if cmdName == "/model" {
						m.textarea.Reset()
						m.setMode(ModeModelSelect)
						return m, nil
					}
					if cmdName == "/agent" {
						m.textarea.Reset()
						m.setMode(ModeAgentSelect)
						return m, nil
					}

					// Dynamic commands
					for _, c := range m.commands {
						if c.Name == cmdName {
							m.textarea.Reset()
							m.setMode(ModeChat)
							m.showList = false
							return m, c.Action(&m, parts[1:])
						}
					}
				}
			}

			// 4. Command List Selection (Fallback)
			if m.showList {
				if i := m.list.SelectedItem(); i != nil {
					if cmd, ok := i.(CommandItem); ok {
						m.textarea.Reset()
						m.setMode(ModeChat)
						m.showList = false
						return m, cmd.Action(&m, strings.Fields(cmd.Name)[1:])
					}
				}
			}

			// 5. Input Submission
			v = m.textarea.Value()
			if v == "" {
				return m, nil
			}

			// Shell Mode
			if m.mode == ModeShell {
				cmdToRun := strings.TrimPrefix(v, "!")
				m.conversation(v, true)
				m.textarea.Reset()
				m.thinking = true
				return m, m.runShellCommand(cmdToRun)
			}

			// Chat Message
			m.conversation(v, true)
			m.thinking = true
			m.conversation(fmt.Sprintf("Processing with %s (Model: %s)...", m.currentAgent, m.currentModel), false)
			m.textarea.Reset()
			m.viewport.GotoBottom()
			m.setMode(ModeChat)
			return m, nil

		case key.Matches(msg, m.keys.Slash):
			if m.mode == ModeChat && m.textarea.Value() == "" {
				m.setMode(ModeCmd)
				m.showList = true
				m.list.ResetSelected()
			}

		case key.Matches(msg, m.keys.Bang):
			if m.mode == ModeChat && m.textarea.Value() == "" {
				m.setMode(ModeShell)
			}
		}

		// Global List Visibility Logic for Text Input
		if m.mode != ModeModelSelect && m.mode != ModeAgentSelect {
			v := m.textarea.Value()
			if strings.HasPrefix(v, "/") {
				m.showList = true
				if m.mode != ModeCmd {
					m.mode = ModeCmd // Switch to Cmd mode for styling
					m.textarea.Prompt = "/ "
				}
				// Note: filtering happens in Update logic above next cycle or immediately if we structure loop differently
				// But we did it above.
			} else if strings.HasPrefix(v, "!") {
				if m.mode != ModeShell {
					m.mode = ModeShell
					m.textarea.Prompt = "! "
				}
			} else if len(v) == 0 && m.mode != ModeChat {
				m.setMode(ModeChat)
				m.showList = false
			}
		}
	}

	return m, tea.Batch(tiCmd, vpCmd, listCmd, spinCmd)
}

func (m *InteractiveModel) toggleList() {
	m.showList = !m.showList
	if m.showList {
		m.list.ResetSelected()
		// Always reset items when toggling manually
		if m.mode == ModeCmd || m.mode == ModeChat {
			m.setListItemsToCommands()
		}
	}
}

func (m *InteractiveModel) setMode(mode InputMode) {
	m.mode = mode
	m.list.ResetFilter() // Ensure no internal filter active

	switch mode {
	case ModeChat:
		m.textarea.Focus() // Ensure focus returns to chat input
		m.textarea.Prompt = "┃ "
		m.textarea.Placeholder = "Type a message (or / for commands)..."
		m.textarea.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("205")) // Pink
		m.showList = false
		m.setListItemsToCommands() // Reset to full command list for next time

	case ModeCmd:
		m.textarea.Prompt = "/ "
		m.textarea.Placeholder = "Type to filter commands..."
		m.textarea.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Grey
		m.showList = true
		m.setListItemsToCommands()

	case ModeShell:
		m.textarea.Prompt = "! "
		m.textarea.Placeholder = "Shell command..."
		m.textarea.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4B400")) // Yellow
		m.showList = false

	case ModeModelSelect:
		m.list.SetShowTitle(true)
		m.textarea.Blur()
		m.showList = true
		m.setListItemsToModels()

	case ModeAgentSelect:
		m.list.SetShowTitle(true)
		m.textarea.Blur()
		m.showList = true
		m.setListItemsToAgents()
	}
}

func (m *InteractiveModel) setListItemsToCommands() {
	items := make([]list.Item, len(m.commands))
	for i, c := range m.commands {
		items[i] = c
	}
	m.list.SetItems(items)
	m.list.Title = "Slash Commands"
	m.list.Styles.Title = interactiveTitleStyle
}

func (m *InteractiveModel) setListItemsToModels() {
	// Dynamically get models based on current agent
	models, ok := m.agentModels[m.currentAgent]
	if !ok {
		// Fallback to Gemini if unknown
		models = m.agentModels["gemini"]
	}

	items := make([]list.Item, len(models))
	for i, mod := range models {
		items[i] = mod
	}
	m.list.SetItems(items)

	// Dynamic Title
	m.list.Title = fmt.Sprintf("Select Model for %s (Enter to confirm)", strings.Title(m.currentAgent))
	m.list.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF")).Background(lipgloss.Color("#4285F4")).Padding(0, 1)
}

func (m *InteractiveModel) setListItemsToAgents() {
	items := make([]list.Item, len(m.agents))
	for i, ag := range m.agents {
		items[i] = ag
	}
	m.list.SetItems(items)
	m.list.Title = "Select Agent Provider (Enter to confirm, Esc to cancel)"
	m.list.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF")).Background(lipgloss.Color("#04B575")).Padding(0, 1) // Green
}

func (m *InteractiveModel) runShellCommand(cmdStr string) tea.Cmd {
	return func() tea.Msg {
		c := exec.Command("sh", "-c", cmdStr)
		out, err := c.CombinedOutput()
		output := string(out)
		if err != nil {
			output += fmt.Sprintf("\nError: %v", err)
		}
		return shellOutputMsg(output)
	}
}

type shellOutputMsg string

// StatusMsg is a message type for displaying status information.
type StatusMsg string

// ErrorMsg is a message indicating an error has occurred.
type ErrorMsg error

// ClearHistory clears the conversation history.
func (m *InteractiveModel) ClearHistory() {
	m.history = []string{}
	clearedMsg := interactiveStatusMessageStyle.Render("Conversation history cleared.")
	m.history = append(m.history, clearedMsg)
	m.viewport.SetContent(strings.Join(m.history, "\n"))
	m.viewport.GotoBottom()
}

func (m InteractiveModel) View() string {
	var views []string

	// Menu Mode - Logo + List
	if m.mode == ModeModelSelect || m.mode == ModeAgentSelect {
		views = append(views, LogoContainerStyle.Render(GenerateLogo()))
		m.list.SetHeight(20)
		views = append(views, interactiveListStyle.Render(m.list.View()))
		views = append(views, helpStyle(m.help.View(m.keys)))
		return interactiveAppStyle.Render(lipgloss.JoinVertical(lipgloss.Left, views...))
	}

	// Normal View
	logo := GenerateLogo()
	views = append(views, LogoContainerStyle.Render(logo))

	// Info Bar
	modeStr := "Chat"
	if m.mode == ModeShell {
		modeStr = "Shell"
	} else if m.mode == ModeCmd {
		modeStr = "Command"
	}
	infoBar := infoBarStyle.Render(fmt.Sprintf("Provider: %s • Model: %s • Mode: %s", strings.Title(m.currentAgent), m.currentModel, modeStr))
	views = append(views, infoBar)

	// Layout Switch: Show List OR Viewport
	if m.showList {
		avail := m.height - 10
		if avail < 10 {
			avail = 10
		}
		m.list.SetHeight(avail)
		views = append(views, interactiveListStyle.Render(m.list.View()))
	} else {
		borderColor := lipgloss.Color("240") // Default Grey
		if m.mode == ModeShell {
			borderColor = lipgloss.Color("#F4B400") // Yellow
		} else if m.mode == ModeChat {
			borderColor = lipgloss.Color("63") // Purple
		}

		vpStyle := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1).
			MarginLeft(2)
		views = append(views, vpStyle.Render(m.viewport.View()))
	}

	views = append(views, promptStyle.Render(m.textarea.View()))

	// Status & Footer
	status := ""
	if m.thinking {
		text := "Thinking..."
		if m.mode == ModeShell {
			text = "Executing..."
		}
		status = m.spinner.View() + " " + text
	}

	footer := lipgloss.JoinHorizontal(lipgloss.Top,
		helpStyle(m.help.View(m.keys)),
		lipgloss.NewStyle().MarginLeft(4).Render(status),
	)
	views = append(views, footer)

	return interactiveAppStyle.Render(lipgloss.JoinVertical(lipgloss.Left, views...))
}

func helpStyle(s string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginLeft(2).MarginTop(1).Render(s)
}

func (m *InteractiveModel) conversation(msg string, isUser bool) {
	style := interactiveBotStyle
	prefix := "Recac: "
	if isUser {
		style = interactiveSenderStyle
		prefix = "You: "
	}
	if m.mode == ModeShell && isUser {
		style = interactiveShellStyle
		prefix = "Shell: "
	}
	newMsg := fmt.Sprintf("%s%s", style.Render(prefix), msg)
	m.history = append(m.history, newMsg)
	m.viewport.SetContent(strings.Join(m.history, "\n"))
	m.viewport.GotoBottom()
}

// Helper to load models from JSON
func loadModelsFromFile(filename string) ([]ModelItem, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var root struct {
		Models []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
			Description string `json:"description"`
		} `json:"models"`
	}

	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	var items []ModelItem
	for _, m := range root.Models {
		// Use DisplayName if available, else Name
		name := m.DisplayName
		if name == "" {
			name = m.Name
		}
		// Use Name as Value (ID)
		value := m.Name

		desc := m.Description
		if desc == "" {
			desc = name // Fallback
		}

		items = append(items, ModelItem{
			Name:               name,
			Value:              value,
			DescriptionDetails: desc,
		})
	}
	return items, nil
}
