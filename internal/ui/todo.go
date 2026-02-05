package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	todoTitleStyle = lipgloss.NewStyle().MarginLeft(2)
)

// TodoItem represents a task in TODO.md
type TodoItem struct {
	Text      string
	Done      bool
	LineIndex int // Index in original file lines
}

func (i TodoItem) Title() string {
	status := "[ ]"
	if i.Done {
		status = "[x]"
	}
	return fmt.Sprintf("%s %s", status, i.Text)
}

func (i TodoItem) Description() string { return "" }
func (i TodoItem) FilterValue() string { return i.Text }

type TodoModel struct {
	list          list.Model
	filename      string
	originalLines []string
	items         []TodoItem
	quitting      bool
	err           error
}

func NewTodoModel(filename string) (TodoModel, error) {
	lines, err := readLines(filename)
	if err != nil {
		// If file doesn't exist, start with empty
		if os.IsNotExist(err) {
			lines = []string{"# TODO", ""}
		} else {
			return TodoModel{}, err
		}
	}

	var items []TodoItem
	var listItems []list.Item

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ] ") {
			item := TodoItem{
				Text:      strings.TrimPrefix(trimmed, "- [ ] "),
				Done:      false,
				LineIndex: i,
			}
			items = append(items, item)
			listItems = append(listItems, item)
		} else if strings.HasPrefix(trimmed, "- [x] ") {
			item := TodoItem{
				Text:      strings.TrimPrefix(trimmed, "- [x] "),
				Done:      true,
				LineIndex: i,
			}
			items = append(items, item)
			listItems = append(listItems, item)
		}
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(listItems, delegate, 0, 0)
	l.Title = "TODO List"
	l.SetShowHelp(true)

	return TodoModel{
		list:          l,
		filename:      filename,
		originalLines: lines,
		items:         items,
	}, nil
}

func (m TodoModel) Init() tea.Cmd {
	return nil
}

func (m TodoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "enter", " ":
			if m.list.SelectedItem() != nil {
				selectedItem := m.list.SelectedItem().(TodoItem)
				index := m.list.Index()

				// Toggle
				selectedItem.Done = !selectedItem.Done
				m.items[index] = selectedItem

				// Update Original Lines
				line := m.originalLines[selectedItem.LineIndex]

				// Simple replacement ensuring we match the checkbox marker
				if selectedItem.Done {
					// Was undone, now done
					m.originalLines[selectedItem.LineIndex] = strings.Replace(line, "- [ ]", "- [x]", 1)
				} else {
					// Was done, now undone
					m.originalLines[selectedItem.LineIndex] = strings.Replace(line, "- [x]", "- [ ]", 1)
				}

				// Save File
				if err := writeLines(m.filename, m.originalLines); err != nil {
					m.err = err
					return m, tea.Quit
				}

				// Update List Item
				newListItems := make([]list.Item, len(m.items))
				for i, item := range m.items {
					newListItems[i] = item
				}
				m.list.SetItems(newListItems)
				m.list.Select(index) // Restore selection

				return m, nil
			}
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m TodoModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}
	if m.quitting {
		return ""
	}
	return m.list.View()
}

// Helper functions

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func writeLines(path string, lines []string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return w.Flush()
}
