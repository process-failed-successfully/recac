package main

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/stretchr/testify/assert"
)

func TestTourModel_Init(t *testing.T) {
	items := []list.Item{
		TourStop{TitleText: "Step 1", Desc: "Desc 1", Content: "Content 1"},
		TourStop{TitleText: "Step 2", Desc: "Desc 2", Content: "Content 2"},
	}
	m := NewTourModel(items)

	// Check initial state
	assert.Equal(t, 2, len(m.list.Items()))
	assert.Equal(t, "Step 1", m.list.SelectedItem().(TourStop).TitleText)
}

func TestTourModel_Update_Navigation(t *testing.T) {
	items := []list.Item{
		TourStop{TitleText: "Step 1", Desc: "Desc 1", Content: "Content 1"},
		TourStop{TitleText: "Step 2", Desc: "Desc 2", Content: "Content 2"},
	}
	m := NewTourModel(items)

	// Use a safe renderer for tests
	var err error
	m.renderer, err = glamour.NewTermRenderer(glamour.WithStandardStyle("notty"))
	assert.NoError(t, err)

	// Initialize ready state (simulate WindowSizeMsg)
	// This sets up the viewport
	updatedM, _ := updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 50})
	m = updatedM

	// Send "down" key (simulating 'j' or Down arrow)
	// bubbletea list default keymap includes 'j' and 'down'
	updatedM, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	m = updatedM

	// Check selection moved to Step 2
	assert.Equal(t, "Step 2", m.list.SelectedItem().(TourStop).TitleText)

	// Check viewport content updated
	// The viewport content is set in Update when selection changes
	assert.Contains(t, m.viewport.View(), "Content 2")
	// Note: "Content 2" might be wrapped or styled, but "Content 2" string should be present if using notty style which is minimal.
}

func TestTourModel_Resize(t *testing.T) {
	items := []list.Item{
		TourStop{TitleText: "Step 1", Desc: "Desc 1", Content: "Content 1"},
	}
	m := NewTourModel(items)

	// Simulate Resize
	updatedM, _ := updateModel(m, tea.WindowSizeMsg{Width: 200, Height: 100})
	m = updatedM

	// Check readiness
	assert.True(t, m.ready)

	// Check list width (30% of 200 = 60)
	assert.Equal(t, 60, m.list.Width())

	// Check viewport width (200 - 60 - 2 = 138)
	assert.Equal(t, 138, m.viewport.Width)
}

// updateModel is a helper to cast the result back to tourModel
func updateModel(m tourModel, msg tea.Msg) (tourModel, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(tourModel), cmd
}
