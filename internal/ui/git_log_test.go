package ui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestGitLogModel(t *testing.T) {
	commits := []CommitItem{
		{Hash: "123", Author: "Me", Date: "Now", Message: "Fix"},
	}

	fetchDiff := func(hash string) (string, error) {
		if hash == "123" {
			return "diff content", nil
		}
		return "", errors.New("diff error")
	}
	explain := func(hash string) (string, error) {
		return "AI explanation", nil
	}
	audit := func(hash string) (string, error) {
		return "AI audit", nil
	}

	m := NewGitLogModel(commits, fetchDiff, explain, audit)

	// Test Init
	assert.Nil(t, m.Init())

	// Test Update: Resize
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = newModel.(GitLogModel)
	assert.Equal(t, 100, m.width)

	// Test Update: Enter (Fetch Diff)
	m.list.Select(0)
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(GitLogModel)
	assert.Equal(t, "Fetching diff...", m.statusMessage)

	// Execute cmd
	msg := cmd()
	dMsg, ok := msg.(diffMsg)
	assert.True(t, ok)
	assert.Equal(t, "diff content", dMsg.content)

	// Test Update: Handle Diff Msg
	newModel, _ = m.Update(dMsg)
	m = newModel.(GitLogModel)
	assert.True(t, m.viewingDetails)
	assert.Equal(t, "", m.statusMessage)

	// Test View (Details)
	m.viewport.Width = 100 // ensure width set for header
	view := m.View()
	assert.Contains(t, view, "Commit Details")
	assert.Contains(t, view, "diff content")

	// Test Update: Esc (Back)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(GitLogModel)
	assert.False(t, m.viewingDetails)

	// Test View (List)
	view = m.View()
	assert.Contains(t, view, "Git Log")
	assert.Contains(t, view, "Fix")

	// Test Update: Explain (e)
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = newModel.(GitLogModel)
	assert.Contains(t, m.statusMessage, "Asking AI")

	msg = cmd()
	aMsg, ok := msg.(analysisResultMsg)
	assert.True(t, ok)
	assert.Equal(t, "AI explanation", aMsg.result)

	// Test Update: Audit (s)
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = newModel.(GitLogModel)
	assert.Contains(t, m.statusMessage, "Auditing")

	msg = cmd()
	aMsg, ok = msg.(analysisResultMsg)
	assert.True(t, ok)
	assert.Equal(t, "AI audit", aMsg.result)

	// Test Analysis Result Msg
	newModel, _ = m.Update(aMsg)
	m = newModel.(GitLogModel)
	assert.True(t, m.viewingDetails)

	// Test Quit
	m.viewingDetails = false
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	msg = cmd()
	assert.IsType(t, tea.QuitMsg{}, msg)
}

func TestCommitItem_Methods(t *testing.T) {
	c := CommitItem{
		Hash:    "abc",
		Author:  "bob",
		Date:    "2023",
		Message: "msg",
	}
	assert.Equal(t, "abc - msg", c.Title())
	assert.Equal(t, "bob | 2023", c.Description())
	assert.Equal(t, "msg abc bob", c.FilterValue())
}
