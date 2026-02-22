package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanFilesForTypo(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some files
	files := map[string]string{
		"file1.txt":    "some text",
		"file2.md":     "markdown text",
		"file3.go":     "go code",
		"file4.bin":    string([]byte{0x00, 0x01}), // Binary file
		"ignored.lock": "lock file",
		".hidden":      "hidden file",
	}

	for name, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644)
		require.NoError(t, err)
	}

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.Mkdir(subDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subDir, "file5.txt"), []byte("subdir text"), 0644)
	require.NoError(t, err)

	// Test scan
	scannedFiles, err := scanFilesForTypo(tmpDir, 100)
	require.NoError(t, err)

	// Check results
	// Should include file1.txt, file2.md, file3.go, subdir/file5.txt
	// Should exclude file4.bin, ignored.lock, .hidden
	assert.Contains(t, scannedFiles, filepath.Join(tmpDir, "file1.txt"))
	assert.Contains(t, scannedFiles, filepath.Join(tmpDir, "file2.md"))
	assert.Contains(t, scannedFiles, filepath.Join(tmpDir, "file3.go"))
	assert.Contains(t, scannedFiles, filepath.Join(subDir, "file5.txt"))

	// Binary file might be tricky if IsBinaryExt relies on extension
	// Let's assume .bin is binary.
	assert.NotContains(t, scannedFiles, filepath.Join(tmpDir, "file4.bin"))
	assert.NotContains(t, scannedFiles, filepath.Join(tmpDir, "ignored.lock"))
	assert.NotContains(t, scannedFiles, filepath.Join(tmpDir, ".hidden"))
}

func TestExtractTypoCandidates(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	err := os.WriteFile(file1, []byte("Hello world! This is a tset."), 0644)
	require.NoError(t, err)

	file2 := filepath.Join(tmpDir, "file2.go")
	err = os.WriteFile(file2, []byte("func main() { fmt.Println(\"Hellow\") }"), 0644)
	require.NoError(t, err)

	files := []string{file1, file2}
	candidates, fileMap := extractTypoCandidates(files)

	// "Hello", "world", "This", "tset", "func", "main", "Println", "Hellow"
	// "func", "main" are likely allowed.
	// "tset" and "Hellow" should be candidates.

	assert.Contains(t, candidates, "Hellow")
	assert.Contains(t, candidates, "tset")

	// Check file map
	assert.Contains(t, fileMap["Hellow"], file2)
	assert.Contains(t, fileMap["tset"], file1)
}

func TestCheckTyposWithAI(t *testing.T) {
	// Mock agent
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		mockAgent := agent.NewMockAgent()
		mockAgent.SetResponse(`{"reciever": "receiver", "funtion": "function"}`)
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	viper.Set("provider", "mock")
	viper.Set("model", "mock-model")

	words := []string{"reciever", "funtion", "correct"}
	typos, err := checkTyposWithAI(context.Background(), words)
	require.NoError(t, err)

	assert.Equal(t, "receiver", typos["reciever"])
	assert.Equal(t, "function", typos["funtion"])
	assert.NotContains(t, typos, "correct")
}

func TestReplaceInFile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	err := os.WriteFile(tmpFile, []byte("This is a reciever test."), 0644)
	require.NoError(t, err)

	err = replaceInFile(tmpFile, "reciever", "receiver")
	require.NoError(t, err)

	content, err := os.ReadFile(tmpFile)
	require.NoError(t, err)
	assert.Equal(t, "This is a receiver test.", string(content))
}

func TestRunTypo(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	err := os.WriteFile(file1, []byte("This has a tset typo."), 0644)
	require.NoError(t, err)

	// Mock agent
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		mockAgent := agent.NewMockAgent()
		// Return JSON with typo fix
		mockAgent.SetResponse(`{"tset": "test"}`)
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	viper.Set("provider", "mock")
	viper.Set("model", "mock-model")

	// Create command
	cmd := &cobra.Command{}
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)

	// Enable auto-fix via flag variable (global var in typo.go)
	// We need to reset it after test as it is global
	oldAutoFix := typoAutoFix
	typoAutoFix = true
	defer func() { typoAutoFix = oldAutoFix }()

	// Run command
	err = runTypo(cmd, []string{tmpDir})
	require.NoError(t, err)

	// Check output
	output := outBuf.String()
	assert.Contains(t, output, "Found 1 typos")
	assert.Contains(t, output, "'tset' -> 'test'")
	assert.Contains(t, output, "Fixed in")

	// Check file content
	content, err := os.ReadFile(file1)
	require.NoError(t, err)
	assert.Contains(t, string(content), "This has a test typo.")
}
