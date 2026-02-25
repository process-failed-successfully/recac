package main

import (
	"context"
	"os"
	"testing"

	"recac/internal/agent"

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

	// We expect "funtion" and "reciever" to be in candidates.
	// We also expect "this", "with", "another", "function" unless they are allowed.
	// "function" is not in the hardcoded allowlist in typo.go? Wait, let me check.
	// typo.go has "func", "return", etc.
	// "function" is > 4 chars.

	// Just check if our target typos are present.
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
