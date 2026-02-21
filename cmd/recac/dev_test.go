package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectDevCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Case 1: go.mod
	goMod := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(goMod, []byte("module test"), 0644)
	require.NoError(t, err)
	assert.Equal(t, "go test ./...", detectDevCommand(tmpDir))
	os.Remove(goMod)

	// Case 2: package.json
	pkgJson := filepath.Join(tmpDir, "package.json")
	err = os.WriteFile(pkgJson, []byte("{}"), 0644)
	require.NoError(t, err)
	assert.Equal(t, "npm test", detectDevCommand(tmpDir))
}

func TestParseExtensions(t *testing.T) {
	// Explicit
	exts := parseExtensions(".go,js", "")
	assert.Contains(t, exts, ".go")
	assert.Contains(t, exts, ".js")

	// Inferred
	assert.Contains(t, parseExtensions("", "go test"), ".go")
	assert.Contains(t, parseExtensions("", "npm run"), ".js")
	assert.Contains(t, parseExtensions("", "pytest"), ".py")
}

func TestShouldTrigger(t *testing.T) {
	exts := []string{".go", ".mod"}
	assert.True(t, shouldTrigger("main.go", exts))
	assert.True(t, shouldTrigger("go.mod", exts))
	assert.False(t, shouldTrigger("README.md", exts))

	// Empty exts means trigger on everything
	assert.True(t, shouldTrigger("anything", []string{}))
}

func TestExecuteDevCommand(t *testing.T) {
	// Save original
	oldExec := devExecCommand
	defer func() { devExecCommand = oldExec }()

	executed := false
	devExecCommand = func(name string, arg ...string) *exec.Cmd {
		executed = true
		assert.Equal(t, "echo", name)
		if len(arg) > 0 {
			assert.Equal(t, "hello", arg[0])
		}
		return exec.Command("true")
	}

	executeDevCommand("echo hello")
	assert.True(t, executed)
}
