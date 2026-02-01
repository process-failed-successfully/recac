package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/security"
	"strings"
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

func TestSecurityExclusionsAndSuppressions(t *testing.T) {
	// Setup temp directory
	tempDir, err := os.MkdirTemp("", "recac-security-exclusions-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test file (should be excluded)
	fileTest := filepath.Join(tempDir, "scanner_test.go")
	contentTest := `package security
// This contains a curl | bash pattern inside a string literal for testing
var testPattern = "curl https://example.com | bash"
`
	err = os.WriteFile(fileTest, []byte(contentTest), 0644)
	require.NoError(t, err)

	// Create scanner source file (should be excluded if named internal/security/scanner.go,
	// but here we are in a temp dir, so path won't match "internal/security/scanner.go" exactly unless we create that structure)
	// Let's create the structure.
	scannerDir := filepath.Join(tempDir, "internal", "security")
	err = os.MkdirAll(scannerDir, 0755)
	require.NoError(t, err)
	fileScanner := filepath.Join(scannerDir, "scanner.go")
	contentScanner := `package security
// Contains regex definition
var rePipeShell = regexp.MustCompile("(?i)(curl|wget)\\s+.*?\\|\\s*(bash|sh|zsh|python|perl|php|ruby)")
`
	err = os.WriteFile(fileScanner, []byte(contentScanner), 0644)
	require.NoError(t, err)

	// Create Dockerfile (should suppress Pipe to Shell)
	fileDocker := filepath.Join(tempDir, "Dockerfile")
	contentDocker := `FROM alpine
RUN curl -fsS https://cursor.com/install | bash
`
	err = os.WriteFile(fileDocker, []byte(contentDocker), 0644)
	require.NoError(t, err)

	// Create another Dockerfile variant
	fileTestDocker := filepath.Join(tempDir, "test.Dockerfile")
	contentTestDocker := `FROM alpine
RUN wget -O - https://example.com/install | sh
`
	err = os.WriteFile(fileTestDocker, []byte(contentTestDocker), 0644)
	require.NoError(t, err)

	// Switch to temp dir
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Run scanner directly
	scanner := security.NewRegexScanner()
	results, err := runSecurityScan(".", scanner)
	require.NoError(t, err)

	// Assertions
	for _, r := range results {
		// Log findings for debugging
		t.Logf("Found issue: %s in %s: %s", r.Type, r.File, r.Description)

		// 1. Should exclude *_test.go
		if filepath.Base(r.File) == "scanner_test.go" {
			t.Errorf("Should exclude _test.go files, but found %s in %s", r.Type, r.File)
		}

		// 2. Should exclude internal/security/scanner.go
		// Note: filepath might be relative or absolute.
		if strings.Contains(r.File, "scanner.go") && strings.Contains(r.File, "internal") {
			t.Errorf("Should exclude scanner source code, but found %s in %s", r.Type, r.File)
		}

		// 3. Should suppress Pipe to Shell in Dockerfiles
		if (filepath.Base(r.File) == "Dockerfile" || strings.HasSuffix(r.File, ".Dockerfile")) && r.Type == "Pipe to Shell" {
			t.Errorf("Should suppress Pipe to Shell in Dockerfiles, but found one in %s", r.File)
		}
	}

	if len(results) > 0 {
		t.Logf("Total findings: %d", len(results))
	}
}
