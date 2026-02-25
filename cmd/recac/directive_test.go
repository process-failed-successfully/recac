package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectiveCommands(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	err := os.Chdir(tmpDir)
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origWd) }()

	instruction := "Use Go 1.25"

	// 1. Test setDirective
	var buf bytes.Buffer
	err = setDirective(&buf, instruction)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Directive set")

	// Verify file exists
	content, err := os.ReadFile(filepath.Join(tmpDir, ".recac", "directive"))
	require.NoError(t, err)
	assert.Equal(t, instruction, string(content))

	// 2. Test showDirective
	var showBuf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&showBuf)
	err = showDirective(cmd)
	require.NoError(t, err)
	assert.Contains(t, showBuf.String(), instruction)

	// 3. Test clearDirective
	var clearBuf bytes.Buffer
	err = clearDirective(&clearBuf)
	require.NoError(t, err)
	assert.Contains(t, clearBuf.String(), "Directive cleared")

	// Verify file deleted
	_, err = os.Stat(filepath.Join(tmpDir, ".recac", "directive"))
	assert.True(t, os.IsNotExist(err))

	// 4. Test clearDirective again (idempotent)
	clearBuf.Reset()
	err = clearDirective(&clearBuf)
	require.NoError(t, err)
	assert.Contains(t, clearBuf.String(), "No global directive to clear")

	// 5. Test showDirective when missing
	showBuf.Reset()
	err = showDirective(cmd)
	require.NoError(t, err)
	assert.Contains(t, showBuf.String(), "No global directive set")
}
