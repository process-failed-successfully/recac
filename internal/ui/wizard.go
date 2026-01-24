package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4")).
		MarginBottom(1)
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	subtleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
)

const (
	StepPath = iota
	StepProvider
	StepMaxAgents
	StepTaskMaxIterations
)

type WizardModel struct {
	textInput         textinput.Model
	list              list.Model
	step              int
	done              bool
	Path              string
	Provider          string
	MaxAgents         int
	TaskMaxIterations int
	errMsg            string
	err               error
}

func NewWizardModel() WizardModel {
	ti := textinput.New()
	ti.Placeholder = "/path/to/project"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40

	// Define Agents/Providers (Reuse AgentItem from interactive.go if possible, or redefine)
	// Since we are in package ui, we can use AgentItem if exported. Checked interactive.go, AgentItem is exported.
	items := []list.Item{
		AgentItem{Name: "Gemini", Value: "gemini", DescriptionDetails: "Google DeepMind Gemini Models"},
		AgentItem{Name: "Gemini CLI", Value: "gemini-cli", DescriptionDetails: "Google Gemini CLI Integration"}, // Highlighting this option
		AgentItem{Name: "OpenAI", Value: "openai", DescriptionDetails: "OpenAI GPT Models"},
		AgentItem{Name: "OpenRouter", Value: "openrouter", DescriptionDetails: "Models via OpenRouter"},
		AgentItem{Name: "Ollama", Value: "ollama", DescriptionDetails: "Local Models via Ollama"},
		AgentItem{Name: "Anthropic", Value: "anthropic", DescriptionDetails: "Anthropic Claude Models"},
		AgentItem{Name: "Cursor CLI", Value: "cursor-cli", DescriptionDetails: "Cursor Editor CLI Integration"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select Agent Provider"
	l.SetShowHelp(false)
	l.SetHeight(10)

	return WizardModel{
		textInput: ti,
		list:      l,
		step:      StepPath,
		done:      false,
	}
}

func (m WizardModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Clear error message on any key press
		if msg.Type != tea.KeyEnter {
			m.errMsg = ""
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.step == StepPath {
				m.Path = m.textInput.Value()
				if m.Path != "" {
					m.step = StepProvider
					// Resize list if needed, or just transition
					return m, nil
				} else {
					m.errMsg = "Path cannot be empty"
				}
			} else if m.step == StepProvider {
				if i := m.list.SelectedItem(); i != nil {
					m.Provider = i.(AgentItem).Value
					m.step = StepMaxAgents
					m.textInput.Reset()
					m.textInput.Placeholder = "1"
					m.textInput.Focus()
					return m, nil
				}
			} else if m.step == StepMaxAgents {
				val := m.textInput.Value()
				if val == "" {
					m.MaxAgents = 1
				} else {
					var n int
					fmt.Sscanf(val, "%d", &n)
					if n < 1 {
						n = 1
					}
					m.MaxAgents = n
				}
				m.step = StepTaskMaxIterations
				m.textInput.Reset()
				m.textInput.Placeholder = "10"
				m.textInput.Focus()
				return m, nil
			} else if m.step == StepTaskMaxIterations {
				val := m.textInput.Value()
				if val == "" {
					m.TaskMaxIterations = 10
				} else {
					var n int
					fmt.Sscanf(val, "%d", &n)
					if n < 1 {
						n = 1
					}
					m.TaskMaxIterations = n
				}
				m.done = true
				return m, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil
	case error:
		m.err = msg
		return m, nil
	}

	if m.step == StepPath || m.step == StepMaxAgents || m.step == StepTaskMaxIterations {
		m.textInput, cmd = m.textInput.Update(msg)
	} else if m.step == StepProvider {
		m.list, cmd = m.list.Update(msg)
	}
	return m, cmd
}

func (m WizardModel) View() string {
	if m.done {
		return fmt.Sprintf("Selected project: %s\nSelected provider: %s\n", m.Path, m.Provider)
	}

	if m.step == StepPath {
		var b strings.Builder
		b.WriteString(titleStyle.Render("Project Setup"))
		b.WriteString("\n\n")
		b.WriteString("Enter project directory:\n")
		b.WriteString(m.textInput.View())
		if m.errMsg != "" {
			b.WriteString("\n")
			b.WriteString(errorStyle.Render(m.errMsg))
		}
		b.WriteString("\n\n(Esc to quit)")
		return b.String()
	} else if m.step == StepProvider {
		return "\n" + m.list.View()
	} else if m.step == StepMaxAgents {
		var b strings.Builder
		b.WriteString(titleStyle.Render("Agent Configuration"))
		b.WriteString("\n\n")
		b.WriteString("Enter maximum parallel agents:\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n")
		b.WriteString(subtleStyle.Render("(Press Enter for default: 1)"))
		b.WriteString("\n\n(Esc to quit)")
		return b.String()
	} else if m.step == StepTaskMaxIterations {
		var b strings.Builder
		b.WriteString(titleStyle.Render("Agent Configuration"))
		b.WriteString("\n\n")
		b.WriteString("Enter maximum iterations per task:\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n")
		b.WriteString(subtleStyle.Render("(Press Enter for default: 10)"))
		b.WriteString("\n\n(Esc to quit)")
		return b.String()
	}

	return ""
}
