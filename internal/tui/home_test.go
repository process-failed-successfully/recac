package tui_test

import (
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
