package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrchestrateFlags(t *testing.T) {
	// Check if watch-dir flag exists
	flag := orchestrateCmd.Flags().Lookup("watch-dir")
	assert.NotNil(t, flag, "watch-dir flag should exist")
}
