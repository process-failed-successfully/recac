package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSessionCmd_Structure(t *testing.T) {
	assert.Equal(t, "session", sessionCmd.Use)
	assert.NotEmpty(t, sessionCmd.Short)
	assert.NotNil(t, sessionCmd.RunE)
}
