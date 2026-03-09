package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// MockTypoAgent implements agent.Agent for testing
type MockTypoAgent struct {
	Response string
}

func (m *MockTypoAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockTypoAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestExtractTypoCandidates(t *testing.T) {
	// Mock readFileFunc
	origReadFile := readFileFunc
	filesMap := map[string]string{
		"file1.txt": "This is a funtion with a reciever.",
		"file2.txt": "Another function.",
	}
	readFileFunc = func(name string) ([]byte, error) {
		if content, ok := filesMap[name]; ok {
			return []byte(content), nil
		}
		return nil, os.ErrNotExist
	}
	defer func() { readFileFunc = origReadFile }()

	files := []string{"file1.txt", "file2.txt"}
	candidates, fileMap := extractTypoCandidates(files)

	foundFuntion := false
	foundReciever := false
	for _, c := range candidates {
		if c == "funtion" {
			foundFuntion = true
		}
		if c == "reciever" {
			foundReciever = true
		}
	}
	assert.True(t, foundFuntion, "funtion should be a candidate")
	assert.True(t, foundReciever, "reciever should be a candidate")

	// Check file map
	assert.Contains(t, fileMap["funtion"], "file1.txt")
}

func TestCheckTyposWithAI(t *testing.T) {
	// Mock Agent Factory
	origAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockTypoAgent{
			Response: `{"funtion": "function", "reciever": "receiver"}`,
		}, nil
	}
	defer func() { agentClientFactory = origAgentFactory }()

	candidates := []string{"funtion", "reciever"}
	typos, err := checkTyposWithAI(context.Background(), candidates)
	assert.NoError(t, err)
	assert.Equal(t, "function", typos["funtion"])
	assert.Equal(t, "receiver", typos["reciever"])
}

func TestReplaceInFile(t *testing.T) {
	// Mock readFileFunc and writeFileFunc
	origReadFile := readFileFunc
	origWriteFile := writeFileFunc

	// Need to restore after test
	defer func() {
		readFileFunc = origReadFile
		writeFileFunc = origWriteFile
	}()

	filesMap := map[string]string{
		"file1.txt": "This is a funtion.",
	}

	readFileFunc = func(name string) ([]byte, error) {
		if content, ok := filesMap[name]; ok {
			return []byte(content), nil
		}
		return nil, os.ErrNotExist
	}

	var capturedPath string
	var capturedContent []byte
	writeFileFunc = func(name string, data []byte, perm os.FileMode) error {
		capturedPath = name
		capturedContent = data
		return nil
	}

	err := replaceInFile("file1.txt", "funtion", "function")
	assert.NoError(t, err)
	assert.Equal(t, "file1.txt", capturedPath)
	assert.Equal(t, "This is a function.", string(capturedContent))
}

func TestRunTypo(t *testing.T) {
	// 1. Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-typo-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a file with a typo
	filePath := filepath.Join(tmpDir, "typo.txt")
	originalContent := "This is a funtion."
	if err := os.WriteFile(filePath, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// 2. Mock Agent Factory
	origAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockTypoAgent{
			Response: `{"funtion": "function"}`,
		}, nil
	}
	defer func() { agentClientFactory = origAgentFactory }()

	// 3. Setup Command
	cmd := &cobra.Command{
		Use: "typo",
		RunE: runTypo,
	}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Set Flags
	typoAutoFix = false
	typoLimit = 100

	// 4. Run Command
	err = runTypo(cmd, []string{tmpDir})
	assert.NoError(t, err)

	// 5. Verify Output
	output := buf.String()
	assert.Contains(t, output, "Scanning")
	assert.Contains(t, output, "Found 1 typos")
	assert.Contains(t, output, "'funtion' -> 'function'")

	// 6. Test Auto-Fix
	typoAutoFix = true
	buf.Reset()

	// Reset file content because runTypo might have touched it? No, typoAutoFix was false.
	// But let's be safe.
	// Actually we want to verify it fixes it now.

	err = runTypo(cmd, []string{tmpDir})
	assert.NoError(t, err)

	output = buf.String()
	assert.Contains(t, output, "Fixed in")

	// Verify file content
	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Equal(t, "This is a function.", string(content))
}
