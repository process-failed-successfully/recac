package main

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTypoExtraction(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.go")
	content := `package main
// This is a comment with a reciever
func main() {
	var myVar string = "some string with typpo"
	// ignored keywords: func, string, var
}
`
	err := os.WriteFile(file, []byte(content), 0644)
	assert.NoError(t, err)

	files := []string{file}
	candidates, _ := extractTypoCandidates(files)

	// Check if "reciever" and "typpo" are in candidates
	// And "package", "main" are NOT
	candidateSet := make(map[string]bool)
	for _, c := range candidates {
		candidateSet[c] = true
	}

	assert.True(t, candidateSet["reciever"], "Should extract 'reciever'")
	assert.True(t, candidateSet["typpo"], "Should extract 'typpo'")
	assert.False(t, candidateSet["func"], "Should ignore 'func'")
	assert.False(t, candidateSet["package"], "Should ignore 'package'")
}

func TestRunTypo(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "bad.go")
	content := `package main
// fixing the reciever
`
	err := os.WriteFile(file, []byte(content), 0644)
	assert.NoError(t, err)

	// Mock Agent
	oldFactory := agentClientFactory
	defer func() { agentClientFactory = oldFactory }()

	mockAgent := new(MockAgent)
	agentClientFactory = func(ctx context.Context, provider, model, cwd, purpose string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Expectation
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(`{"reciever": "receiver"}`, nil)

	// Run command via root to test integration
	output, err := executeCommand(rootCmd, "typo", tmpDir, "--limit", "10")

	assert.NoError(t, err)
	assert.Contains(t, output, "'reciever' -> 'receiver'")
	assert.Contains(t, output, "Found 1 typos")

	mockAgent.AssertExpectations(t)
}

func TestRunTypo_Fix(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "fixable.go")
	content := `package main
// fixing the reciever
`
	err := os.WriteFile(file, []byte(content), 0644)
	assert.NoError(t, err)

	// Mock Agent
	oldFactory := agentClientFactory
	defer func() { agentClientFactory = oldFactory }()

	mockAgent := new(MockAgent)
	agentClientFactory = func(ctx context.Context, provider, model, cwd, purpose string) (agent.Agent, error) {
		return mockAgent, nil
	}
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(`{"reciever": "receiver"}`, nil)

	// Mock askOneFunc
	oldAskOne := askOneFunc
	defer func() { askOneFunc = oldAskOne }()

	askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		// Confirm: true
		if r, ok := response.(*bool); ok {
			*r = true
		}
		return nil
	}

	// Run command via root to test integration
	output, err := executeCommand(rootCmd, "typo", tmpDir, "--limit", "10", "--fix")

	assert.NoError(t, err)
	assert.Contains(t, output, "Fixed in")

	// Verify file content
	newContent, err := os.ReadFile(file)
	assert.NoError(t, err)
	assert.Contains(t, string(newContent), "receiver")
	assert.NotContains(t, string(newContent), "reciever")

	mockAgent.AssertExpectations(t)
}
