package main

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgent is a shared mock
type MockAgentForTypo struct {
	mock.Mock
}

func (m *MockAgentForTypo) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockAgentForTypo) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func TestExtractTypoCandidates(t *testing.T) {
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "f1.txt")
	os.WriteFile(f1, []byte("helo world validWord"), 0644)

	candidates, fileMap := extractTypoCandidates([]string{f1})

	assert.Contains(t, candidates, "helo")
	assert.Contains(t, candidates, "world")
	assert.Contains(t, candidates, "validWord")
	assert.Equal(t, []string{f1}, fileMap["helo"])
}

func TestScanFilesForTypo(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "f1.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "f2.bin"), []byte{0xFF, 0x00}, 0644) // Binary
	os.Mkdir(filepath.Join(tmpDir, ".git"), 0755) // Ignored

	files, err := scanFilesForTypo(tmpDir, 10)
	assert.NoError(t, err)

	// Only f1.txt should be found (f2.bin is binary, .git is skipped)
	// scanFilesForTypo returns absolute paths usually, unless Walk gives relative.
	// Walk gives paths relative to root if root is relative, or absolute if root is absolute.
	// t.TempDir() is absolute.
	assert.Len(t, files, 1)
	assert.Equal(t, filepath.Join(tmpDir, "f1.txt"), files[0])
}

func TestFindFilesWithWord(t *testing.T) {
	m := map[string][]string{
		"word": {"f1", "f2"},
	}
	res := findFilesWithWord(m, "word")
	assert.Equal(t, []string{"f1", "f2"}, res)
}

func TestReplaceInFile(t *testing.T) {
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "f1.txt")
	os.WriteFile(f1, []byte("Hello wrld"), 0644)

	err := replaceInFile(f1, "wrld", "world")
	assert.NoError(t, err)

	content, _ := os.ReadFile(f1)
	assert.Equal(t, "Hello world", string(content))
}

func TestCheckTyposWithAI(t *testing.T) {
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := new(MockAgentForTypo)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(`{"helo": "hello"}`, nil)

	agentClientFactory = func(ctx context.Context, provider, model, workspace, projectID string) (agent.Agent, error) {
		return mockAgent, nil
	}

	typos, err := checkTyposWithAI(context.Background(), []string{"helo"})
	assert.NoError(t, err)
	assert.Equal(t, "hello", typos["helo"])
}
