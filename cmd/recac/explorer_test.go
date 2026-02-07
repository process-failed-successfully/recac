package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExplorerCommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "explorer" {
			found = true
			break
		}
	}
	assert.True(t, found, "explorer command should be registered")
}
