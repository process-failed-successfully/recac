package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFocusCmd_Flags(t *testing.T) {
	// Reset flags
	focusDuration = "25m"
	focusBreak = "5m"
	focusTask = ""

	// Check flags
	f := focusCmd.Flags()

	d, err := f.GetString("duration")
	assert.NoError(t, err)
	assert.Equal(t, "25m", d)

	b, err := f.GetString("break")
	assert.NoError(t, err)
	assert.Equal(t, "5m", b)

	tk, err := f.GetString("task")
	assert.NoError(t, err)
	assert.Equal(t, "", tk)
}

func TestFocusCmd_InvalidDuration(t *testing.T) {
	// Set invalid duration
	focusDuration = "invalid"

	// Execute RunE directly
	err := focusCmd.RunE(focusCmd, []string{})

	// Should fail at time.ParseDuration
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid duration format")

	// Reset
	focusDuration = "25m"
}

func TestFocusCmd_Structure(t *testing.T) {
	assert.Equal(t, "focus", focusCmd.Use)
	assert.NotEmpty(t, focusCmd.Short)
	assert.NotEmpty(t, focusCmd.Long)
}
