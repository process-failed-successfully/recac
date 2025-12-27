package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4")).
		MarginBottom(1)
)

type WizardModel struct {
        textInput textinput.Model
        done      bool
        Path      string
        err       error
}

func NewWizardModel() WizardModel {
        ti := textinput.New()
        ti.Placeholder = "/path/to/project"
        ti.Focus()
        ti.CharLimit = 156
        ti.Width = 40

        return WizardModel{
                textInput: ti,
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
                switch msg.Type {
                case tea.KeyCtrlC, tea.KeyEsc:
                        return m, tea.Quit
                case tea.KeyEnter:
                        m.Path = m.textInput.Value()
                        m.done = true
                        return m, tea.Quit
                }
        case error:
                m.err = msg
                return m, nil
        }

        m.textInput, cmd = m.textInput.Update(msg)
        return m, cmd
}

func (m WizardModel) View() string {
        if m.done {
                return fmt.Sprintf("Selected project: %s\n", m.Path)
        }
	var b strings.Builder
	b.WriteString(titleStyle.Render("Project Setup"))
	b.WriteString("\n\n")
	b.WriteString("Enter project directory:\n")
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n(Esc to quit)")

	return b.String()
}
