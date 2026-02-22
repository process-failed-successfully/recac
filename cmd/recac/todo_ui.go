package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"recac/internal/utils"
)

var todoUiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Interactive TUI for managing TODOs",
	Long:  `Launch an interactive terminal user interface to manage, solve, and navigate TODOs in TODO.md.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureTodoFile(); err != nil {
			return err
		}
		items, err := getTodoItems()
		if err != nil {
			return err
		}

		m := newTodoUiModel(items)
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	todoCmd.AddCommand(todoUiCmd)
}

// Model

type todoUiModel struct {
	list          list.Model
	viewport      viewport.Model
	showViewport  bool
	ready         bool
	solving       bool
	keys          *keyMap
	width, height int
}

// Item

type todoItem struct {
	raw      string // Full line content
	title    string // Task description
	desc     string // File location if any
	filePath string
	lineNum  int
	done     bool
	index    int // 1-based index in TODO.md (counting tasks only)
}

func (i todoItem) Title() string {
	check := "[ ]"
	if i.done {
		check = "[x]"
	}
	return fmt.Sprintf("%s %s", check, i.title)
}

func (i todoItem) Description() string { return i.desc }
func (i todoItem) FilterValue() string { return i.title }

// KeyMap

type keyMap struct {
	Toggle  key.Binding
	Delete  key.Binding
	Solve   key.Binding
	Preview key.Binding
	Quit    key.Binding
}

var keys = keyMap{
	Toggle: key.NewBinding(
		key.WithKeys("enter", "space"),
		key.WithHelp("enter/space", "toggle status"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d", "delete"),
		key.WithHelp("d", "delete task"),
	),
	Solve: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "solve with AI"),
	),
	Preview: key.NewBinding(
		key.WithKeys("p", "right", "l"),
		key.WithHelp("p/→", "preview file"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

// Logic

func newTodoUiModel(items []list.Item) todoUiModel {
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "TODOs"
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			keys.Toggle,
			keys.Delete,
			keys.Solve,
			keys.Preview,
		}
	}
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			keys.Toggle,
			keys.Preview,
		}
	}

	return todoUiModel{
		list: l,
		keys: &keys,
	}
}

func (m todoUiModel) Init() tea.Cmd {
	return nil
}

func (m todoUiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

		vpHeight := msg.Height - v
		vpWidth := msg.Width / 2

		if !m.ready {
			m.viewport = viewport.New(vpWidth, vpHeight)
			m.viewport.HighPerformanceRendering = false
			m.ready = true
		} else {
			m.viewport.Width = vpWidth
			m.viewport.Height = vpHeight
		}

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}

		if key.Matches(msg, m.keys.Toggle) {
			if i, ok := m.list.SelectedItem().(todoItem); ok {
				newStatus := !i.done
				err := toggleTaskStatus(i.index, newStatus)
				if err != nil {
					cmd = m.list.NewStatusMessage(fmt.Sprintf("Error: %v", err))
					return m, cmd
				}
				cmd = m.list.NewStatusMessage(fmt.Sprintf("Task %d toggled", i.index))
				return m, tea.Batch(cmd, reloadListCmd())
			}
			return m, nil
		}

		if key.Matches(msg, m.keys.Delete) {
			if i, ok := m.list.SelectedItem().(todoItem); ok {
				err := removeTask(i.index)
				if err != nil {
					cmd = m.list.NewStatusMessage(fmt.Sprintf("Error: %v", err))
					return m, cmd
				}
				cmd = m.list.NewStatusMessage(fmt.Sprintf("Task %d deleted", i.index))
				return m, tea.Batch(cmd, reloadListCmd())
			}
			return m, nil
		}

		if key.Matches(msg, m.keys.Solve) {
			if i, ok := m.list.SelectedItem().(todoItem); ok {
				if i.filePath == "" {
					cmd = m.list.NewStatusMessage("No file context for this task!")
					return m, cmd
				}
				m.solving = true
				cmd = m.list.NewStatusMessage(fmt.Sprintf("Solving task %d with AI...", i.index))
				return m, tea.Batch(cmd, solveTaskCmd(i.index))
			}
			return m, nil
		}

		if key.Matches(msg, m.keys.Preview) {
			if i, ok := m.list.SelectedItem().(todoItem); ok {
				if m.showViewport {
					m.showViewport = false
					h, _ := docStyle.GetFrameSize()
					m.list.SetSize(m.width-h, m.list.Height())
					return m, nil
				}

				if i.filePath != "" {
					content, err := os.ReadFile(i.filePath)
					if err == nil {
						m.viewport.SetContent(string(content))
						lines := strings.Split(string(content), "\n")
						yOffset := 0
						if i.lineNum > 0 && i.lineNum <= len(lines) {
							yOffset = i.lineNum - (m.viewport.Height / 2)
							if yOffset < 0 {
								yOffset = 0
							}
						}
						m.viewport.YOffset = yOffset
						m.showViewport = true

						h, _ := docStyle.GetFrameSize()
						m.list.SetSize((m.width/2)-h, m.list.Height())

					} else {
						cmd = m.list.NewStatusMessage("Could not read file: " + err.Error())
						return m, cmd
					}
				} else {
					cmd = m.list.NewStatusMessage("No file context")
					return m, cmd
				}
			}
			return m, nil
		}

	case solveResultMsg:
		m.solving = false
		if msg.err != nil {
			cmd = m.list.NewStatusMessage(fmt.Sprintf("Solve Error: %v", msg.err))
			return m, cmd
		} else {
			cmd = m.list.NewStatusMessage("Task solved successfully!")
			return m, tea.Batch(cmd, reloadListCmd())
		}

	case itemsLoadedMsg:
		if msg.err != nil {
			cmd = m.list.NewStatusMessage("Failed to reload list: " + msg.err.Error())
			return m, cmd
		}
		cmd = m.list.SetItems(msg.items)
		return m, cmd
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	if m.showViewport {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

var docStyle = lipgloss.NewStyle().Margin(1, 2)

func (m todoUiModel) View() string {
	if m.showViewport {
		listStyle := lipgloss.NewStyle().Width(m.width/2 - 2).MarginRight(1)
		vpStyle := lipgloss.NewStyle().Width(m.width/2 - 2).MarginLeft(1)

		return lipgloss.JoinHorizontal(lipgloss.Top,
			listStyle.Render(m.list.View()),
			vpStyle.Render(m.viewport.View()),
		)
	}

	return docStyle.Render(m.list.View())
}

// Commands and Messages

type solveResultMsg struct {
	err error
}

func solveTaskCmd(index int) tea.Cmd {
	return func() tea.Msg {
		err := solveTodoTask(context.Background(), index, io.Discard)
		return solveResultMsg{err: err}
	}
}

type itemsLoadedMsg struct {
	items []list.Item
	err   error
}

func reloadListCmd() tea.Cmd {
	return func() tea.Msg {
		items, err := getTodoItems()
		return itemsLoadedMsg{items, err}
	}
}

// Helpers

func getTodoItems() ([]list.Item, error) {
	if err := ensureTodoFile(); err != nil {
		return nil, err
	}
	lines, err := utils.ReadLines(todoFile)
	if err != nil {
		return nil, err
	}

	var items []list.Item
	index := 1
	re := regexp.MustCompile(`\[([^]]+):(\d+)\]`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]") {
			done := strings.HasPrefix(trimmed, "- [x]")

			prefix := "- [ ] "
			if done {
				prefix = "- [x] "
			}

			content := strings.TrimPrefix(trimmed, prefix)

			filePath := ""
			lineNum := 0
			matches := re.FindStringSubmatch(content)
			desc := ""

			if len(matches) >= 3 {
				filePath = matches[1]
				lineNum, _ = strconv.Atoi(matches[2])
				desc = fmt.Sprintf("File: %s:%d", filePath, lineNum)
				// Clean up title by removing the [file:line] part
				content = strings.TrimSpace(strings.Replace(content, matches[0], "", 1))
			}

			items = append(items, todoItem{
				raw:      line,
				title:    content,
				desc:     desc,
				filePath: filePath,
				lineNum:  lineNum,
				done:     done,
				index:    index,
			})
			index++
		}
	}
	return items, nil
}
