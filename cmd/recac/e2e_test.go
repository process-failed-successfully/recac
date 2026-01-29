package main

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestE2ECmd_Structure(t *testing.T) {
	cmd := NewE2ECmd()
	assert.Equal(t, "e2e", cmd.Use)
	assert.NotEmpty(t, cmd.Short)

	// Verify subcommands
	subCmds := cmd.Commands()
	assert.NotEmpty(t, subCmds)

	expectedSubCmds := []string{"setup", "deploy", "wait", "verify", "cleanup"}
	foundSubCmds := make(map[string]bool)

	for _, sub := range subCmds {
		foundSubCmds[sub.Name()] = true
		// Verify DisableFlagParsing is set to true for delegation
		assert.True(t, sub.DisableFlagParsing, "DisableFlagParsing should be true for subcommand %s", sub.Name())
	}

	for _, name := range expectedSubCmds {
		assert.True(t, foundSubCmds[name], "Subcommand %s not found", name)
	}
}

func TestE2ECmd_RunSetup(t *testing.T) {
	// Mock function
	called := false
	var capturedArgs []string

	origSetupFunc := runSetupFunc
	runSetupFunc = func(args []string) error {
		called = true
		capturedArgs = args
		return nil
	}
	defer func() { runSetupFunc = origSetupFunc }()

	cmd := NewE2ECmd()
	// Find setup subcommand
	var setupCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "setup" {
			setupCmd = c
			break
		}
	}
	assert.NotNil(t, setupCmd)

	// Execute
	// Since DisableFlagParsing is true, we need to pass args via RunE arguments if called manually
	err := setupCmd.RunE(setupCmd, []string{"--scenario", "test"})
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, []string{"--scenario", "test"}, capturedArgs)
}

func TestE2ECmd_RunSetup_Error(t *testing.T) {
	// Mock function
	origSetupFunc := runSetupFunc
	runSetupFunc = func(args []string) error {
		return errors.New("mock error")
	}
	defer func() { runSetupFunc = origSetupFunc }()

	cmd := NewE2ECmd()
	var setupCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "setup" {
			setupCmd = c
			break
		}
	}

	err := setupCmd.RunE(setupCmd, []string{})
	assert.Error(t, err)
	assert.Equal(t, "mock error", err.Error())
}
