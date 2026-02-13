package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityCmd(t *testing.T) {
	// Setup temp directory
	tempDir, err := os.MkdirTemp("", "recac-security-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a file with a fake secret (Generic API Key)
	file1 := filepath.Join(tempDir, "config.py")
	content1 := `
def connect():
    api_key = "abcdefghijklmnopqrstuvwxyz123456" # MATCH
    print("connecting...")
`
	err = os.WriteFile(file1, []byte(content1), 0644)
	require.NoError(t, err)

	// Create a file with a dangerous command
	file2 := filepath.Join(tempDir, "script.sh")
	content2 := `
#!/bin/bash
cat /etc/passwd # MATCH
`
	err = os.WriteFile(file2, []byte(content2), 0755)
	require.NoError(t, err)

	// Create a clean file
	file3 := filepath.Join(tempDir, "clean.go")
	content3 := `package main
func main() {
	println("Hello")
}
`
	err = os.WriteFile(file3, []byte(content3), 0644)
	require.NoError(t, err)

	// Create a file in ignored directory
	gitDir := filepath.Join(tempDir, ".git")
	err = os.Mkdir(gitDir, 0755)
	require.NoError(t, err)
	fileIgnored := filepath.Join(gitDir, "secrets.txt")
	err = os.WriteFile(fileIgnored, []byte("api_key = 'ignored_secret_key_1234567890'"), 0644)
	require.NoError(t, err)

	// Switch to temp dir so the command runs there
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Helper to reset flags
	resetFlags := func() {
		securityJSON = false
		securityFail = false
	}

	t.Run("Security Scan Text Output", func(t *testing.T) {
		resetFlags()
		cmd := securityCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := cmd.RunE(cmd, []string{})
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "Generic API Token")
		assert.Contains(t, output, "config.py")
		assert.Contains(t, output, "Dangerous Command")
		assert.Contains(t, output, "script.sh")

		// Should not match ignored file
		assert.NotContains(t, output, "secrets.txt")
		// Should not match clean file
		assert.NotContains(t, output, "clean.go")
	})

	t.Run("Security Scan JSON Output", func(t *testing.T) {
		resetFlags()
		securityJSON = true
		cmd := securityCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := cmd.RunE(cmd, []string{})
		require.NoError(t, err)

		output := buf.String()
		var results []SecurityResult
		err = json.Unmarshal([]byte(output), &results)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, len(results), 2)

		foundToken := false
		foundDangerous := false

		for _, r := range results {
			if r.Type == "Generic API Token" {
				foundToken = true
			}
			if r.Type == "Dangerous Command" {
				foundDangerous = true
			}
		}
		assert.True(t, foundToken)
		assert.True(t, foundDangerous)
	})

	t.Run("Security Scan Fail Flag", func(t *testing.T) {
		resetFlags()
		securityFail = true
		cmd := securityCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := cmd.RunE(cmd, []string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security scan failed")
	})

	t.Run("No Issues", func(t *testing.T) {
		resetFlags()

		// create a clean subdir
		cleanDir := filepath.Join(tempDir, "clean_subdir")
		os.Mkdir(cleanDir, 0755)

		os.Chdir(cleanDir)
		defer os.Chdir(tempDir)

		cmd := securityCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := cmd.RunE(cmd, []string{})
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "No security issues found")
	})
}
