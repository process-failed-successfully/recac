package main

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestGetCommands(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	child1 := &cobra.Command{Use: "child1", Short: "Child 1"}
	child2 := &cobra.Command{Use: "child2", Short: "Child 2"}
	hidden := &cobra.Command{Use: "hidden", Hidden: true}
	deprecated := &cobra.Command{Use: "deprecated", Deprecated: "use something else"}

	root.AddCommand(child1, child2, hidden, deprecated)

	items := getCommands(root)

	assert.Len(t, items, 2)
	assert.Equal(t, "child1", items[0].(item).Title())
	assert.Equal(t, "child2", items[1].(item).Title())
}

func TestUIModel_Navigation(t *testing.T) {
	// Setup a command tree
	root := &cobra.Command{Use: "root"}
	parent := &cobra.Command{Use: "parent"}
	child := &cobra.Command{Use: "child"}
	root.AddCommand(parent)
	parent.AddCommand(child)

	items := getCommands(root)
	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)

	m := uiModel{
		list:       l,
		currentCmd: root,
		history:    []*cobra.Command{},
	}

	// Simulate selecting "parent" (index 0)
	m.list.Select(0)

	// Send "enter" key
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, cmd := m.Update(msg)

	// Assertions
	m = newModel.(uiModel)
	assert.Equal(t, parent, m.currentCmd, "Current command should be 'parent'")
	assert.Len(t, m.history, 1, "History should have 1 item")
	assert.Equal(t, root, m.history[0], "History item should be 'root'")
	assert.Len(t, m.list.Items(), 1, "List should have 1 item (child)")
	assert.Equal(t, "child", m.list.Items()[0].(item).Title(), "List item should be 'child'")
	assert.Nil(t, cmd, "Cmd should be nil for navigation")

	// Simulate "backspace"
	msg = tea.KeyMsg{Type: tea.KeyBackspace}
	newModel, cmd = m.Update(msg)

	m = newModel.(uiModel)

	// Verify we went back
	assert.Equal(t, root, m.currentCmd, "Current command should be 'root'")
	assert.Len(t, m.history, 0, "History should be empty")
	assert.Len(t, m.list.Items(), 1, "List should have 1 item (parent)")
	assert.Equal(t, "parent", m.list.Items()[0].(item).Title(), "List item should be 'parent'")
	assert.Nil(t, cmd, "Cmd should be nil for backspace")
}
