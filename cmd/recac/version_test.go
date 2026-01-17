package main

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionCmd(t *testing.T) {
	cmd := NewVersionCmd()
	b := new(bytes.Buffer)
	cmd.SetOut(b)

	err := cmd.Execute()
	assert.NoError(t, err)

	out := b.String()
	assert.Contains(t, out, "recac version")
	assert.Contains(t, out, "Commit:")
	assert.Contains(t, out, "Build Date:")
	assert.Contains(t, out, "Go Version:")
	assert.Contains(t, out, "Platform:")
	assert.Contains(t, out, runtime.Version())
	assert.Contains(t, out, runtime.GOOS)
	assert.Contains(t, out, runtime.GOARCH)

	// Check if version variables are correctly used
	assert.True(t, strings.Contains(out, version), "Output should contain version")
	assert.True(t, strings.Contains(out, commit), "Output should contain commit")
	assert.True(t, strings.Contains(out, date), "Output should contain date")
}
