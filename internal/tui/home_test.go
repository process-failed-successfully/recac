package tui_test

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"recac/internal/tui"
	"strings"
	"testing"
	"time"
)

func TestHomeModel_View(t *testing.T) {
	git := tui.GitStatus{
		Branch:         "main",
		Dirty:          true,
		LastCommitMsg:  "Initial commit",
		LastCommitHash: "abcdef1",
	}
	todos := tui.TodoSummary{
		Count:    10,
		Critical: 2,
	}
	sessions := []tui.RecentSession{
		{Name: "session-1", Status: "running", Time: time.Now()},
	}
	sys := tui.SystemInfo{OS: "linux"}

	m := tui.NewHomeModel(git, todos, sessions, sys)
	view := m.View()

	if !strings.Contains(view, "Branch: main") {
		t.Error("View missing branch name")
	}
	if !strings.Contains(view, "Dirty: Yes") {
		t.Error("View missing dirty status")
	}
	if !strings.Contains(view, "Total TODOs: 10") {
		t.Error("View missing todo count")
	}
	if !strings.Contains(view, "Critical: 2") {
		t.Error("View missing critical todo count")
	}
	if !strings.Contains(view, "session-1") {
		t.Error("View missing session name")
	}
}

func TestHomeModel_Init(t *testing.T) {
	m := tui.NewHomeModel(tui.GitStatus{}, tui.TodoSummary{}, nil, tui.SystemInfo{})
	assert.Nil(t, m.Init())
}

func TestHomeModel_Update(t *testing.T) {
	m := tui.NewHomeModel(tui.GitStatus{}, tui.TodoSummary{}, nil, tui.SystemInfo{})

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	assert.NotNil(t, cmd)

	newM, cmd = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	assert.Nil(t, cmd)
	m2 := newM.(tui.HomeModel)
	assert.Equal(t, 100, m2.Width)
	assert.Equal(t, 50, m2.Height)
}
