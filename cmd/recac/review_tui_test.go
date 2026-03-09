package main

import (
	"testing"
	"os"
	"path/filepath"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestInitialReviewModel(t *testing.T) {
	issues := []ReviewIssue{
		{
			Title:       "Test Issue",
			Description: "Test Desc",
			File:        "test.go",
			Line:        10,
			Severity:    "High",
		},
	}

	m := initialReviewModel(issues)

	assert.Equal(t, 1, len(m.issues))
	assert.Equal(t, "Test Issue", m.issues[0].Title)
	assert.NotNil(t, m.list)
}

func TestReviewModel_Update_Quit(t *testing.T) {
	m := initialReviewModel([]ReviewIssue{})
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}

	newM, cmd := m.Update(msg)

	assert.NotNil(t, cmd)
	// Check if the command returns tea.QuitMsg
	msgCmd := cmd()
	assert.IsType(t, tea.QuitMsg{}, msgCmd)
	assert.Equal(t, m.ready, newM.(ReviewModel).ready)
}

func TestReviewModel_Update_WindowSize(t *testing.T) {
	m := initialReviewModel([]ReviewIssue{})
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}

	newM, _ := m.Update(msg)
	updatedM := newM.(ReviewModel)

	assert.True(t, updatedM.ready)
	assert.Equal(t, 100, updatedM.width)
	assert.Equal(t, 50, updatedM.height)
}

func TestReviewModel_Update_Enter(t *testing.T) {
	issues := []ReviewIssue{
		{
			Title:       "Issue 1",
			Description: "Desc 1",
			File:        "file1.go",
		},
		{
			Title:       "Issue 2",
			Description: "Desc 2",
			File:        "file2.go",
		},
	}
	m := initialReviewModel(issues)

	// Simulate window size to init viewport
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = newM.(ReviewModel)

	// Simulate selecting an item (default is 0)
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newM2, _ := m.Update(msg)
	updatedM := newM2.(ReviewModel)

	assert.NotNil(t, updatedM.selectedIssue)
	assert.Equal(t, "Issue 1", updatedM.selectedIssue.Title)
}

func TestReviewModel_Update_FilteringIgnore(t *testing.T) {
	m := initialReviewModel([]ReviewIssue{
		{Title: "Test Issue"},
	})

	// Set state to filtering
	m.list.SetFilterState(list.Filtering)

	// Send Enter key, which should normally select the issue
	// But since we are filtering, it should be ignored by our switch
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newM, _ := m.Update(msg)

	// Assert no issue was selected
	assert.Nil(t, newM.(ReviewModel).selectedIssue)
}

func TestReviewModel_TableDriven(t *testing.T) {
    tests := []struct {
		name         string
		setupFunc    func(t *testing.T) (ReviewModel, tea.Msg)
		wantReady    bool
        wantQuit     bool
	}{
        {
			name: "ReviewModel Update Enter Table",
			setupFunc: func(t *testing.T) (ReviewModel, tea.Msg) {
                issues := []ReviewIssue{
                    {Title: "Issue 1", Description: "Desc 1", File: "file1.go"},
                }
                m := initialReviewModel(issues)
                newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
                return newM.(ReviewModel), tea.KeyMsg{Type: tea.KeyEnter}
            },
			wantReady: true,
            wantQuit: false,
		},
        {
            name: "ReviewModel Update Quit Table",
			setupFunc: func(t *testing.T) (ReviewModel, tea.Msg) {
                m := initialReviewModel([]ReviewIssue{})
                return m, tea.KeyMsg{Type: tea.KeyCtrlC}
            },
			wantReady: false,
            wantQuit: true,
        },
    }

    for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
            m, msg := tc.setupFunc(t)
            newM, cmd := m.Update(msg)

            updatedM := newM.(ReviewModel)

            assert.Equal(t, tc.wantReady, updatedM.ready)

            if tc.wantQuit {
                msgCmd := cmd()
	            assert.IsType(t, tea.QuitMsg{}, msgCmd)
            }
        })
    }
}

func TestReviewModel_ItemMethods(t *testing.T) {
	issue := ReviewIssue{
		Title:       "Test Issue",
		File:        "test.go",
		Line:        10,
		Severity:    "High",
	}
	itm := item{issue: issue}

	assert.Equal(t, "Test Issue", itm.Title())
	assert.Equal(t, "test.go:10 [High]", itm.Description())
	assert.Equal(t, "Test Issue", itm.FilterValue())
}

func TestReviewModel_InitView(t *testing.T) {
	m := initialReviewModel([]ReviewIssue{})
	cmd := m.Init()
	assert.Nil(t, cmd)

    view := m.View()
	assert.Contains(t, view, "Initializing...")

	// Make it ready
	m.ready = true
	m.statusMessage = "Test Status"
	view = m.View()
	assert.Contains(t, view, "Test Status")
}

func TestReviewModel_ApplyFixCmd(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.go")
	os.WriteFile(tmpFile, []byte("package main\n\nfunc main() {}\n"), 0644)

	issue := &ReviewIssue{
		Title:       "Test Fix",
		File:        tmpFile,
		Line:        3,
		Replacement: "func main() {\n\t// test\n}\n",
	}

	cmd := applyFixCmd(issue)
	msg := cmd()

	assert.IsType(t, fixMsg{}, msg)

	content, _ := os.ReadFile(tmpFile)
	assert.Contains(t, string(content), "// test")
}
