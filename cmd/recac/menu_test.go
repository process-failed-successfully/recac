package main

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestCollectCommands(t *testing.T) {
	// Setup a mock root command structure
	root := &cobra.Command{Use: "app"}

	// Commands must be runnable to be considered "available" unless they have subcommands
	dummyRun := func(cmd *cobra.Command, args []string) {}

	child1 := &cobra.Command{Use: "child1", Short: "First child", Run: dummyRun}
	child2 := &cobra.Command{Use: "child2", Short: "Second child", Run: dummyRun}
	hidden := &cobra.Command{Use: "hidden", Hidden: true, Run: dummyRun}

	grandChild := &cobra.Command{Use: "grand", Short: "Grandchild", Run: dummyRun}

	child1.AddCommand(grandChild)
	root.AddCommand(child1, child2, hidden)

	// Execute
	commands := collectCommands(root)

	// Assert
	assert.Len(t, commands, 3) // child1, child2, grand

	// Expected order based on CommandPath:
	// app child1
	// app child1 grand
	// app child2

	assert.Equal(t, "child1", commands[0].Use)
	assert.Equal(t, "grand", commands[1].Use)
	assert.Equal(t, "child2", commands[2].Use)

	// Verify hidden is ignored
	for _, c := range commands {
		assert.NotEqual(t, "hidden", c.Use)
	}
}

func TestMenuModelInitialization(t *testing.T) {
	cmd1 := &cobra.Command{Use: "test1", Short: "desc1"}
	cmd2 := &cobra.Command{Use: "test2", Short: "desc2"}

	// commandItem is defined in menu.go, so we can use it directly since we are in package main
	items := []list.Item{
		commandItem{title: cmd1.Use, description: cmd1.Short, cmd: cmd1},
		commandItem{title: cmd2.Use, description: cmd2.Short, cmd: cmd2},
	}

	m := newMenuModel(items)

	assert.NotNil(t, m.list)
	assert.Equal(t, "recac commands", m.list.Title)
	assert.Equal(t, 2, len(m.list.Items()))
}
