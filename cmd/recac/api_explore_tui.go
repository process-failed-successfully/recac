package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)

	paneStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	activePaneStyle = paneStyle.Copy().
			BorderForeground(lipgloss.Color("205")) // Pink

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)
)

type ApiExplorerModel struct {
	// State
	endpoints []DiscoveredEndpoint
	loading   bool
	err       error
	scanning  bool

	// Focus
	focusIndex int // 0: List, 1: URL, 2: Headers, 3: Body, 4: Response (scroll)

	// Components
	list      table.Model
	urlInput  textinput.Model
	headers   textarea.Model
	body      textarea.Model
	response  viewport.Model

	// Current Request State
	currentMethod string

	// Window size
	width  int
	height int
}

type endpointsScannedMsg struct {
	Endpoints []DiscoveredEndpoint
	Err       error
}

type apiResponseMsg struct {
	Body       string
	StatusCode int
	Duration   time.Duration
	Err        error
}

func InitialApiExplorerModel() ApiExplorerModel {
	// Initialize Table
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Method", Width: 8},
			{Title: "Path", Width: 30},
			{Title: "Description", Width: 40},
		}),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	// Initialize Inputs
	url := textinput.New()
	url.Placeholder = "/api/..."
	url.Width = 50

	headers := textarea.New()
	headers.Placeholder = "Key: Value"
	headers.SetHeight(5)
	headers.ShowLineNumbers = false

	body := textarea.New()
	body.Placeholder = "{ ... }"
	body.SetHeight(8)
	body.ShowLineNumbers = true

	vp := viewport.New(0, 0) // Dimensions set on WindowSize

	return ApiExplorerModel{
		list:      t,
		urlInput:  url,
		headers:   headers,
		body:      body,
		response:  vp,
		loading:   true, // Start by loading/scanning
		scanning:  true,
		focusIndex: 0,
	}
}

func (m ApiExplorerModel) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		scanEndpointsCmd,
	)
}

func (m ApiExplorerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global keys
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Tab navigation
		if msg.String() == "tab" {
			m.focusIndex = (m.focusIndex + 1) % 5
			m.updateFocus()
			return m, nil
		}
		if msg.String() == "shift+tab" {
			m.focusIndex = (m.focusIndex - 1 + 5) % 5
			m.updateFocus()
			return m, nil
		}

		// Specific actions based on focus
		if m.focusIndex == 0 { // List
			if msg.String() == "enter" {
				m.selectEndpoint()
				// Move focus to URL
				m.focusIndex = 1
				m.updateFocus()
				return m, nil
			}
		}

		if m.focusIndex == 1 || m.focusIndex == 2 || m.focusIndex == 3 {
			// In inputs, Ctrl+Enter or something to send?
			if msg.String() == "ctrl+s" {
				return m, m.sendRequestCmd()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeComponents(msg.Width, msg.Height)

	case endpointsScannedMsg:
		m.scanning = false
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.endpoints = msg.Endpoints
			m.populateTable()
			if len(m.endpoints) > 0 {
				m.list.SetCursor(0)
				m.selectEndpoint() // Populate details for first item
			}
		}

	case apiResponseMsg:
		m.loading = false
		if msg.Err != nil {
			m.response.SetContent(fmt.Sprintf("Error: %v", msg.Err))
		} else {
			content := fmt.Sprintf("Status: %d\nDuration: %s\n\n%s", msg.StatusCode, msg.Duration, msg.Body)
			m.response.SetContent(content)
		}
	}

	// Update components
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	m.urlInput, cmd = m.urlInput.Update(msg)
	cmds = append(cmds, cmd)

	m.headers, cmd = m.headers.Update(msg)
	cmds = append(cmds, cmd)

	m.body, cmd = m.body.Update(msg)
	cmds = append(cmds, cmd)

	m.response, cmd = m.response.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m ApiExplorerModel) View() string {
	if m.scanning {
		return fmt.Sprintf("\n  Scanning codebase for API endpoints... %s\n", m.spinnerView())
	}

	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press Ctrl+C to quit.\n", m.err)
	}

	// Left Pane: List
	listStyle := paneStyle
	if m.focusIndex == 0 {
		listStyle = activePaneStyle
	}
	listView := listStyle.Render(m.list.View())

	// Right Pane: Details
	// URL Bar
	urlStyle := paneStyle
	if m.focusIndex == 1 {
		urlStyle = activePaneStyle
	}
	urlView := urlStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			headerStyle.Render(m.currentMethod),
			m.urlInput.View(),
		),
	)

	// Headers
	headersStyle := paneStyle
	if m.focusIndex == 2 {
		headersStyle = activePaneStyle
	}
	headersView := headersStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			headerStyle.Render("Headers (Key: Value)"),
			m.headers.View(),
		),
	)

	// Body
	bodyStyle := paneStyle
	if m.focusIndex == 3 {
		bodyStyle = activePaneStyle
	}
	bodyView := bodyStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			headerStyle.Render("Body (JSON)"),
			m.body.View(),
		),
	)

	// Response
	respStyle := paneStyle
	if m.focusIndex == 4 {
		respStyle = activePaneStyle
	}
	respView := respStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			headerStyle.Render("Response"),
			m.response.View(),
		),
	)

	// Layout
	// Left: 30% width
	// Right: 70% width

	rightPane := lipgloss.JoinVertical(lipgloss.Left,
		urlView,
		lipgloss.JoinHorizontal(lipgloss.Top, headersView, bodyView),
		respView,
	)

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, rightPane)
}

// Helpers

func (m *ApiExplorerModel) updateFocus() {
	m.list.Blur()
	m.urlInput.Blur()
	m.headers.Blur()
	m.body.Blur()
	// Viewport doesn't have focus state like inputs

	switch m.focusIndex {
	case 0:
		m.list.Focus()
	case 1:
		m.urlInput.Focus()
	case 2:
		m.headers.Focus()
	case 3:
		m.body.Focus()
	case 4:
		// Response viewport (scroll keys work naturally if updated)
	}
}

func (m *ApiExplorerModel) selectEndpoint() {
	selectedRow := m.list.SelectedRow()
	if selectedRow == nil {
		return
	}

	// Find the endpoint
	// This relies on order match which is fragile but works for now as table is populated in order
	idx := m.list.Cursor()
	if idx >= 0 && idx < len(m.endpoints) {
		ep := m.endpoints[idx]
		m.currentMethod = ep.Method
		m.urlInput.SetValue(ep.Path)

		// Headers
		var sb strings.Builder
		for k, v := range ep.Headers {
			sb.WriteString(fmt.Sprintf("%s: %s\n", k, v))
		}
		m.headers.SetValue(sb.String())

		// Body
		m.body.SetValue(ep.Body)
	}
}

func (m *ApiExplorerModel) populateTable() {
	rows := []table.Row{}
	for _, ep := range m.endpoints {
		rows = append(rows, table.Row{
			ep.Method,
			ep.Path,
			ep.Description,
		})
	}
	m.list.SetRows(rows)
}

func (m *ApiExplorerModel) resizeComponents(width, height int) {
	// Simple layout math
	leftWidth := int(float64(width) * 0.3)
	rightWidth := width - leftWidth - 4 // borders/padding

	m.list.SetWidth(leftWidth)
	m.list.SetHeight(height - 4)

	m.urlInput.Width = rightWidth - 4

	halfRightWidth := (rightWidth / 2) - 2
	m.headers.SetWidth(halfRightWidth)
	m.body.SetWidth(halfRightWidth)

	respHeight := height - 15 // subtract other components
	if respHeight < 5 {
		respHeight = 5
	}
	m.response.Width = rightWidth
	m.response.Height = respHeight
}

func (m ApiExplorerModel) spinnerView() string {
	// Simple text spinner
	return "..."
}

// Commands

func scanEndpointsCmd() tea.Msg {
	ctx := context.Background()
	cwd, _ := os.Getwd()
	eps, err := scanEndpointsWithAI(ctx, cwd)
	return endpointsScannedMsg{Endpoints: eps, Err: err}
}

func (m ApiExplorerModel) sendRequestCmd() tea.Cmd {
	return func() tea.Msg {
		method := m.currentMethod
		url := m.urlInput.Value()

		// Parse headers
		headers := make(map[string]string)
		lines := strings.Split(m.headers.Value(), "\n")
		for _, line := range lines {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}

		body := m.body.Value()

		// Execute
		respBody, statusCode, duration, err := executeApiRequest(method, url, headers, body)
		return apiResponseMsg{
			Body:       respBody,
			StatusCode: statusCode,
			Duration:   duration,
			Err:        err,
		}
	}
}

func StartApiExplorer() error {
	m := InitialApiExplorerModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
