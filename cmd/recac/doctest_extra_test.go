package main

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateBash(t *testing.T) {
	// Check if bash is available
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not found")
	}

	t.Run("Valid Bash", func(t *testing.T) {
		err := validateBash("echo hello")
		assert.NoError(t, err)
	})

	t.Run("Invalid Bash", func(t *testing.T) {
		// syntax error: missing fi, incomplete
		err := validateBash("if [ ]; then echo hi")
		assert.Error(t, err)
	})
}

func TestValidateBlock(t *testing.T) {
	// Test YAML
	assert.NoError(t, validateBlock("yaml", "key: value"))
	assert.Error(t, validateBlock("yaml", ": :"))

	// Test Unknown
	assert.NoError(t, validateBlock("unknown", "content"))
}
