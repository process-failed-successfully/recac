package main

import (
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func TestTourModel_Update_Navigation(t *testing.T) {
	steps := []TourStep{
		{Title: "Step 1", Description: "Desc 1"},
		{Title: "Step 2", Description: "Desc 2"},
		{Title: "Step 3", Description: "Desc 3"},
	}

	model := &TourModel{
		steps:    steps,
		cursor:   0,
		focus:    0, // List focus
		viewport: viewport.New(20, 10),
		ready:    true,
	}

	// Test Initial State
	if model.cursor != 0 {
		t.Errorf("Expected cursor 0, got %d", model.cursor)
	}

	// Test Down Navigation
	model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if model.cursor != 1 {
		t.Errorf("Expected cursor 1 after Down, got %d", model.cursor)
	}

	// Test Up Navigation
	model.Update(tea.KeyMsg{Type: tea.KeyUp})
	if model.cursor != 0 {
		t.Errorf("Expected cursor 0 after Up, got %d", model.cursor)
	}

	// Test Bounds (Up at top)
	model.Update(tea.KeyMsg{Type: tea.KeyUp})
	if model.cursor != 0 {
		t.Errorf("Expected cursor 0 after Up at top, got %d", model.cursor)
	}

	// Test Bounds (Down at bottom)
	model.cursor = 2
	model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if model.cursor != 2 {
		t.Errorf("Expected cursor 2 after Down at bottom, got %d", model.cursor)
	}
}

func TestTourModel_Update_Focus(t *testing.T) {
	model := &TourModel{
		steps:    []TourStep{{Title: "S1"}},
		focus:    0,
		viewport: viewport.New(20, 10),
		ready:    true,
	}

	// Test Tab to switch focus
	model.Update(tea.KeyMsg{Type: tea.KeyTab})
	if model.focus != 1 {
		t.Errorf("Expected focus 1 after Tab, got %d", model.focus)
	}

	// Test Tab to switch back
	model.Update(tea.KeyMsg{Type: tea.KeyTab})
	if model.focus != 0 {
		t.Errorf("Expected focus 0 after second Tab, got %d", model.focus)
	}
}

func TestTourModel_Update_DetailNavigation(t *testing.T) {
	model := &TourModel{
		steps:    []TourStep{{Title: "S1"}, {Title: "S2"}},
		cursor:   0,
		focus:    1, // Detail focus
		viewport: viewport.New(20, 10),
		ready:    true,
	}

	// Test Down Key in Detail Focus (should NOT move list cursor)
	model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if model.cursor != 0 {
		t.Errorf("Expected cursor 0 after Down in Detail focus, got %d", model.cursor)
	}
}

func TestTourModel_View(t *testing.T) {
	model := &TourModel{
		steps:    []TourStep{{Title: "S1"}},
		cursor:   0,
		focus:    0,
		viewport: viewport.New(20, 10),
		width:    80,
		height:   24,
		ready:    true,
	}

	view := model.View()
	if view == "" {
		t.Error("Expected non-empty view")
	}
}
