package ui

import (
	"fmt"
	"recac/internal/runner"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Define a placeholder for the ISessionManager interface to avoid circular dependencies.
// The actual implementation will be passed in from the `cmd` package.
type ISessionManager interface {
	RenameSession(oldName, newName string) error
	DeleteSession(name string) error
}

var (
	psAppStyle        = lipgloss.NewStyle().Padding(1, 2)
	psSuccessStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	psErrorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	psPromptStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true)
	psTextInputPrompt = "> "
)

type sessionItem struct {
	runner.SessionState
}

func (i sessionItem) Title() string       { return i.Name }
func (i sessionItem) Description() string { return fmt.Sprintf("Status: %s", i.Status) }
func (i sessionItem) FilterValue() string { return i.Name }

// --- TUI Model ---

type mode int

const (
	modeNavigating mode = iota
	modeRenaming
	modeConfirmingDelete
)

type InteractivePsModel struct {
	list         list.Model
	sm           ISessionManager
	textInput    textinput.Model
	mode         mode
	message      string
	messageStyle lipgloss.Style
}

func NewInteractivePsModel(sm ISessionManager, sessions []runner.SessionState) InteractivePsModel {
	items := make([]list.Item, len(sessions))
	for i, s := range sessions {
		items[i] = sessionItem{s}
	}

	// Setup text input for renaming
	ti := textinput.New()
	ti.Prompt = psTextInputPrompt
	ti.CharLimit = 50
	ti.Width = 50

	delegate := newItemDelegate()
	sessionList := list.New(items, delegate, 0, 0)
	sessionList.Title = "Interactive Session Explorer"
	sessionList.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("230")).
		Padding(0, 1)

	// Override default quit key to prevent exit while renaming
	sessionList.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		}
	}

	return InteractivePsModel{
		list:      sessionList,
		sm:        sm,
		textInput: ti,
		mode:      modeNavigating,
	}
}

func (m InteractivePsModel) Init() tea.Cmd {
	return tea.Batch(tea.EnterAltScreen, textinput.Blink)
}

// --- Update Logic ---

func (m InteractivePsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeRenaming:
		return m.updateRenaming(msg)
	case modeConfirmingDelete:
		return m.updateConfirmingDelete(msg)
	default:
		return m.updateNavigating(msg)
	}
}

func (m *InteractivePsModel) setStatusMessage(msg string, style lipgloss.Style, clearAfter time.Duration) tea.Cmd {
	m.message = msg
	m.messageStyle = style
	return tea.Tick(clearAfter, func(t time.Time) tea.Msg {
		return clearMessageMsg{}
	})
}

// Custom messages for async operations
type (
	sessionRenamedMsg   struct{ oldName, newName string }
	sessionArchivedMsg  struct{ name string }
	sessionDeletedMsg   struct{ index int }
	clearMessageMsg     struct{}
	operationErrorMsg   struct{ err error }
	startRenamingMsg    struct{}
	startConfirmDeleteMsg struct{}
)

func (m InteractivePsModel) updateNavigating(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := psAppStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	// Custom messages
	case startRenamingMsg:
		selectedItem, ok := m.list.SelectedItem().(sessionItem)
		if ok {
			m.mode = modeRenaming
			m.textInput.SetValue(selectedItem.Name)
			m.textInput.Focus()
			return m, nil
		}
	case startConfirmDeleteMsg:
		if _, ok := m.list.SelectedItem().(sessionItem); ok {
			m.mode = modeConfirmingDelete
			cmd := m.setStatusMessage("Are you sure you want to delete this session? (y/N)", psPromptStyle, 20*time.Second)
			return m, cmd
		}

	case sessionRenamedMsg:
		// Find and update the item in the list
		for i, item := range m.list.Items() {
			if item.(sessionItem).Name == msg.oldName {
				updatedItem := item.(sessionItem)
				updatedItem.Name = msg.newName
				cmd := m.list.SetItem(i, updatedItem)
				cmds = append(cmds, cmd)
				break
			}
		}
		cmd := m.setStatusMessage(fmt.Sprintf("Renamed '%s' to '%s'", msg.oldName, msg.newName), psSuccessStyle, 3*time.Second)
		cmds = append(cmds, cmd)

	case sessionArchivedMsg:
		// In a real app, you might want to move this to an "archived" list or remove it
		cmd := m.setStatusMessage(fmt.Sprintf("Archived session '%s'", msg.name), psSuccessStyle, 3*time.Second)
		cmds = append(cmds, cmd)

	case sessionDeletedMsg:
		m.list.RemoveItem(msg.index)
		cmd := m.setStatusMessage("Session deleted", psSuccessStyle, 3*time.Second)
		cmds = append(cmds, cmd)

	case operationErrorMsg:
		cmd := m.setStatusMessage(fmt.Sprintf("Error: %v", msg.err), psErrorStyle, 5*time.Second)
		cmds = append(cmds, cmd)

	case clearMessageMsg:
		m.message = ""
	}

	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m InteractivePsModel) updateRenaming(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "enter":
			newName := m.textInput.Value()
			selectedItem, ok := m.list.SelectedItem().(sessionItem)
			if ok && newName != "" && newName != selectedItem.Name {
				oldName := selectedItem.Name
				m.mode = modeNavigating
				m.textInput.Blur()
				return m, func() tea.Msg {
					err := m.sm.RenameSession(oldName, newName)
					if err != nil {
						return operationErrorMsg{err}
					}
					return sessionRenamedMsg{oldName: oldName, newName: newName}
				}
			}
		case "esc":
			m.mode = modeNavigating
			m.textInput.Blur()
			return m, nil
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m InteractivePsModel) updateConfirmingDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "y", "Y":
			m.mode = modeNavigating
			m.message = ""
			if selectedItem, ok := m.list.SelectedItem().(sessionItem); ok {
				selectedIndex := m.list.Index()
				return m, func() tea.Msg {
					err := m.sm.DeleteSession(selectedItem.Name)
					if err != nil {
						return operationErrorMsg{err}
					}
					return sessionDeletedMsg{index: selectedIndex}
				}
			}
		default: // Any other key cancels
			m.mode = modeNavigating
			m.message = ""
		}
	case clearMessageMsg: // Timeout for confirmation
		m.mode = modeNavigating
		m.message = ""
	}
	return m, nil
}

// --- View Logic ---

func (m InteractivePsModel) View() string {
	var mainContent string
	if m.mode == modeRenaming {
		mainContent = m.textInput.View()
	} else {
		mainContent = m.list.View()
	}

	return psAppStyle.Render(mainContent + "\n" + m.messageStyle.Render(m.message))
}

// --- List Delegate ---

func newItemDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	// This is where we capture key presses on list items
	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		if _, ok := m.SelectedItem().(sessionItem); !ok {
			return nil
		}

		if msg, ok := msg.(tea.KeyMsg); ok {
			switch keypress := msg.String(); keypress {
			case "r":
				return func() tea.Msg { return startRenamingMsg{} }
			case "x":
				return func() tea.Msg { return startConfirmDeleteMsg{} }
			}
		}
		return nil
	}

	// Define the help text that appears for each item
	help := []key.Binding{
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename")),
		key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "delete")),
	}

	d.ShortHelpFunc = func() []key.Binding {
		return help
	}
	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{help}
	}

	return d
}
