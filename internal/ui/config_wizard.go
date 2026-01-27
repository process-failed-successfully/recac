package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	ConfigStepProvider = iota
	ConfigStepAPIKey
	ConfigStepJiraEnabled
	ConfigStepJiraURL
	ConfigStepJiraEmail
	ConfigStepJiraToken
	ConfigStepConfirm
)

// ConfigData holds the collected configuration
type ConfigData struct {
	Provider     string
	APIKey       string
	JiraEnabled  bool
	JiraURL      string
	JiraEmail    string
	JiraToken    string
	Confirmed    bool
}

// ConfigAgentItem implements list.Item for the provider menu
type ConfigAgentItem struct {
	Name               string
	Value              string
	DescriptionDetails string
}

func (i ConfigAgentItem) FilterValue() string { return i.Name }
func (i ConfigAgentItem) Title() string       { return i.Name }
func (i ConfigAgentItem) Description() string { return i.DescriptionDetails }

var configTitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#7D56F4")).
	MarginBottom(1)

type ConfigWizardModel struct {
	textInput textinput.Model
	list      list.Model
	step      int
	Data      ConfigData
	width     int
	height    int
	err       error
}

func NewConfigWizardModel() ConfigWizardModel {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40

	items := []list.Item{
		ConfigAgentItem{Name: "Gemini", Value: "gemini", DescriptionDetails: "Google DeepMind Gemini Models"},
		ConfigAgentItem{Name: "OpenAI", Value: "openai", DescriptionDetails: "OpenAI GPT Models"},
		ConfigAgentItem{Name: "OpenRouter", Value: "openrouter", DescriptionDetails: "Models via OpenRouter"},
		ConfigAgentItem{Name: "Ollama", Value: "ollama", DescriptionDetails: "Local Models via Ollama"},
		ConfigAgentItem{Name: "Anthropic", Value: "anthropic", DescriptionDetails: "Anthropic Claude Models"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select AI Provider"
	l.SetShowHelp(false)
	l.SetHeight(12)

	return ConfigWizardModel{
		textInput: ti,
		list:      l,
		step:      ConfigStepProvider,
		Data:      ConfigData{},
	}
}

func (m ConfigWizardModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m ConfigWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			// Process input based on step
			switch m.step {
			case ConfigStepProvider:
				if i := m.list.SelectedItem(); i != nil {
					m.Data.Provider = i.(ConfigAgentItem).Value
					m.step = ConfigStepAPIKey
					m.textInput.Reset()
					m.textInput.EchoMode = textinput.EchoPassword
					m.textInput.Placeholder = "Enter API Key"
					m.textInput.Focus()
					return m, nil
				}

			case ConfigStepAPIKey:
				m.Data.APIKey = m.textInput.Value()
				m.step = ConfigStepJiraEnabled
				m.textInput.Reset()
				m.textInput.EchoMode = textinput.EchoNormal
				m.textInput.Placeholder = "Enable Jira Integration? (y/N)"
				return m, nil

			case ConfigStepJiraEnabled:
				val := strings.ToLower(strings.TrimSpace(m.textInput.Value()))
				if val == "y" || val == "yes" {
					m.Data.JiraEnabled = true
					m.step = ConfigStepJiraURL
					m.textInput.Reset()
					m.textInput.Placeholder = "https://your-domain.atlassian.net"
				} else {
					m.Data.JiraEnabled = false
					m.step = ConfigStepConfirm // Skip to confirm
				}
				return m, nil

			case ConfigStepJiraURL:
				m.Data.JiraURL = m.textInput.Value()
				m.step = ConfigStepJiraEmail
				m.textInput.Reset()
				m.textInput.Placeholder = "user@example.com"
				return m, nil

			case ConfigStepJiraEmail:
				m.Data.JiraEmail = m.textInput.Value()
				m.step = ConfigStepJiraToken
				m.textInput.Reset()
				m.textInput.EchoMode = textinput.EchoPassword
				m.textInput.Placeholder = "Jira API Token"
				return m, nil

			case ConfigStepJiraToken:
				m.Data.JiraToken = m.textInput.Value()
				m.step = ConfigStepConfirm
				return m, nil

			case ConfigStepConfirm:
				m.Data.Confirmed = true
				return m, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetWidth(msg.Width)
	}

	// Update components
	if m.step == ConfigStepProvider {
		m.list, cmd = m.list.Update(msg)
	} else if m.step != ConfigStepConfirm {
		m.textInput, cmd = m.textInput.Update(msg)
	}

	return m, cmd
}

func (m ConfigWizardModel) View() string {
	if m.Data.Confirmed {
		return "Configuration saved!\n"
	}

	var b strings.Builder
	title := configTitleStyle.Render("Recac Configuration Wizard")
	b.WriteString(title + "\n\n")

	switch m.step {
	case ConfigStepProvider:
		return "\n" + m.list.View()

	case ConfigStepAPIKey:
		b.WriteString(fmt.Sprintf("Enter API Key for %s:\n", m.Data.Provider))
		b.WriteString(m.textInput.View())

	case ConfigStepJiraEnabled:
		b.WriteString("Enable Jira Integration? (y/N):\n")
		b.WriteString(m.textInput.View())

	case ConfigStepJiraURL:
		b.WriteString("Enter Jira URL:\n")
		b.WriteString(m.textInput.View())

	case ConfigStepJiraEmail:
		b.WriteString("Enter Jira Email:\n")
		b.WriteString(m.textInput.View())

	case ConfigStepJiraToken:
		b.WriteString("Enter Jira API Token:\n")
		b.WriteString(m.textInput.View())

	case ConfigStepConfirm:
		b.WriteString("Confirm Settings:\n\n")
		b.WriteString(fmt.Sprintf("Provider: %s\n", m.Data.Provider))
		b.WriteString(fmt.Sprintf("API Key: %s\n", maskString(m.Data.APIKey)))
		b.WriteString(fmt.Sprintf("Jira Enabled: %v\n", m.Data.JiraEnabled))
		if m.Data.JiraEnabled {
			b.WriteString(fmt.Sprintf("Jira URL: %s\n", m.Data.JiraURL))
			b.WriteString(fmt.Sprintf("Jira Email: %s\n", m.Data.JiraEmail))
			b.WriteString(fmt.Sprintf("Jira Token: %s\n", maskString(m.Data.JiraToken)))
		}
		b.WriteString("\nPress Enter to save, Esc to cancel.")
	}

	b.WriteString("\n\n(Esc to quit)")
	return b.String()
}

func maskString(s string) string {
	if len(s) < 4 {
		return "****"
	}
	return s[:2] + "****" + s[len(s)-2:]
}
