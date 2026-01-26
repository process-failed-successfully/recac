package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestCollectCommands(t *testing.T) {
	root := &cobra.Command{Use: "root"}

	child1 := &cobra.Command{Use: "child1", Run: func(c *cobra.Command, args []string) {}}
	child2 := &cobra.Command{Use: "child2", Run: func(c *cobra.Command, args []string) {}}
	grandchild := &cobra.Command{Use: "grandchild", Run: func(c *cobra.Command, args []string) {}}
	hidden := &cobra.Command{Use: "hidden", Hidden: true, Run: func(c *cobra.Command, args []string) {}}
	notRunnable := &cobra.Command{Use: "group"} // No Run, so not runnable

	root.AddCommand(child1, child2, hidden, notRunnable)
	notRunnable.AddCommand(grandchild)

	cmds := collectCommands(root)

	assert.Len(t, cmds, 3) // child1, child2, grandchild

	names := make([]string, len(cmds))
	for i, c := range cmds {
		names[i] = c.Name()
	}

	assert.Contains(t, names, "child1")
	assert.Contains(t, names, "child2")
	assert.Contains(t, names, "grandchild")
	assert.NotContains(t, names, "hidden")
	assert.NotContains(t, names, "group")
}
