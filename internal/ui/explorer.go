package ui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AnalysisFunc defines a function that analyzes a file and returns a result string.
type AnalysisFunc func(path string) (string, error)

// FileItem represents a file or directory in the list.
type FileItem struct {
	Name    string
	Path    string
	IsDir   bool
	Info    fs.FileInfo
	DescStr string
}

func (i FileItem) Title() string {
	if i.IsDir {
		return fmt.Sprintf("📁 %s", i.Name)
	}
	return fmt.Sprintf("📄 %s", i.Name)
}

func (i FileItem) Description() string {
	return i.DescStr
}

func (i FileItem) FilterValue() string { return i.Name }

// ExplorerModel is the main Bubble Tea model for the explorer.
type ExplorerModel struct {
	list           list.Model
	viewport       viewport.Model
	currentPath    string
	viewingFile    bool   // If true, showing viewport
	statusMessage  string // For temporary status like "Analyzing..."

	explainFunc    AnalysisFunc
	complexityFunc AnalysisFunc
	securityFunc   AnalysisFunc

	width  int
	height int
}

// NewExplorerModel creates a new explorer model starting at the given path.
func NewExplorerModel(startPath string, explain, complexity, security AnalysisFunc) (ExplorerModel, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return ExplorerModel{}, err
	}

	m := ExplorerModel{
		currentPath:    absPath,
		explainFunc:    explain,
		complexityFunc: complexity,
		securityFunc:   security,
	}

	// Initialize List
	delegate := list.NewDefaultDelegate()
	m.list = list.New(nil, delegate, 0, 0)
	m.list.Title = "Explorer"
	m.list.SetShowHelp(true)
	m.list.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
			key.NewBinding(key.WithKeys("backspace"), key.WithHelp("backspace", "up")),
		}
	}
	m.list.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open file/dir")),
			key.NewBinding(key.WithKeys("backspace"), key.WithHelp("backspace", "go up")),
			key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "explain (AI)")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "complexity")),
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "security")),
		}
	}

	// Initialize Viewport
	m.viewport = viewport.New(0, 0)

	// Load initial items
	if err := m.loadItems(); err != nil {
		return ExplorerModel{}, err
	}

	return m, nil
}

// loadItems reads the directory and updates the list.
func (m *ExplorerModel) loadItems() error {
	entries, err := os.ReadDir(m.currentPath)
	if err != nil {
		return err
	}

	// Sort: Directories first, then files
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})

	var items []list.Item
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}

		desc := fmt.Sprintf("%s | %s", formatSize(info.Size()), info.ModTime().Format("2006-01-02 15:04"))
		if e.IsDir() {
			desc = "Directory | " + info.ModTime().Format("2006-01-02 15:04")
		}

		items = append(items, FileItem{
			Name:    e.Name(),
			Path:    filepath.Join(m.currentPath, e.Name()),
			IsDir:   e.IsDir(),
			Info:    info,
			DescStr: desc,
		})
	}

	m.list.SetItems(items)
	m.list.Title = fmt.Sprintf("Explorer: %s", m.currentPath)
	return nil
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// --- Bubble Tea Implementation ---

type analysisMsg struct {
	result string
	err    error
}

func (m ExplorerModel) Init() tea.Cmd {
	return nil
}

func (m ExplorerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height)
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.viewingFile {
			switch msg.String() {
			case "q", "esc", "backspace":
				m.viewingFile = false
				m.statusMessage = ""
				return m, nil
			default:
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		}

		// List Mode
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			item := m.list.SelectedItem()
			if item == nil {
				return m, nil
			}
			f := item.(FileItem)

			if f.IsDir {
				m.currentPath = f.Path
				if err := m.loadItems(); err != nil {
					m.statusMessage = fmt.Sprintf("Error: %v", err)
				}
			} else {
				// Open file
				content, err := os.ReadFile(f.Path)
				if err != nil {
					m.statusMessage = fmt.Sprintf("Error reading file: %v", err)
				} else {
					m.viewingFile = true
					m.viewport.SetContent(string(content))
					m.viewport.GotoTop()
					m.statusMessage = "Viewing: " + f.Name
				}
			}

		case "backspace":
			parent := filepath.Dir(m.currentPath)
			if parent != m.currentPath {
				m.currentPath = parent
				if err := m.loadItems(); err != nil {
					m.statusMessage = fmt.Sprintf("Error: %v", err)
				}
			}

		case "e": // Explain
			item := m.list.SelectedItem()
			if item != nil && !item.(FileItem).IsDir && m.explainFunc != nil {
				path := item.(FileItem).Path
				m.statusMessage = "🤖 Asking AI to explain..."
				return m, m.runAnalysis(m.explainFunc, path)
			}

		case "c": // Complexity
			item := m.list.SelectedItem()
			if item != nil && !item.(FileItem).IsDir && m.complexityFunc != nil {
				path := item.(FileItem).Path
				m.statusMessage = "Calculating complexity..."
				return m, m.runAnalysis(m.complexityFunc, path)
			}

		case "s": // Security
			item := m.list.SelectedItem()
			if item != nil && !item.(FileItem).IsDir && m.securityFunc != nil {
				path := item.(FileItem).Path
				m.statusMessage = "Scanning for security issues..."
				return m, m.runAnalysis(m.securityFunc, path)
			}
		}

	case analysisMsg:
		m.statusMessage = ""
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.viewingFile = true
			m.viewport.SetContent(msg.result)
			m.viewport.GotoTop()
		}
	}

	if !m.viewingFile {
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m ExplorerModel) runAnalysis(fn AnalysisFunc, path string) tea.Cmd {
	return func() tea.Msg {
		res, err := fn(path)
		return analysisMsg{result: res, err: err}
	}
}

func (m ExplorerModel) View() string {
	if m.viewingFile {
		return fmt.Sprintf("%s\n%s", m.headerView(), m.viewport.View())
	}
	return fmt.Sprintf("%s\n%s", m.statusView(), m.list.View())
}

func (m ExplorerModel) headerView() string {
	title := "File View"
	line := strings.Repeat("─", max(0, m.viewport.Width-len(title)))
	return lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(title + line)
}

func (m ExplorerModel) statusView() string {
	if m.statusMessage == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(m.statusMessage)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
