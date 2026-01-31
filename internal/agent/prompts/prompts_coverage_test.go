package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListPrompts(t *testing.T) {
	prompts, err := ListPrompts()
	assert.NoError(t, err)
	assert.Contains(t, prompts, Planner)
	assert.Contains(t, prompts, ManagerReview)
	assert.Contains(t, prompts, CodingAgent)
}

func TestGetPrompt_LocalOverride(t *testing.T) {
	// Create .recac/prompts in CWD
	cwd, _ := os.Getwd()
	localDir := filepath.Join(cwd, ".recac", "prompts")
	os.MkdirAll(localDir, 0755)
	defer os.RemoveAll(filepath.Join(cwd, ".recac"))

	promptName := "planner"
	overrideContent := "Local Override Content"
	os.WriteFile(filepath.Join(localDir, promptName+".md"), []byte(overrideContent), 0644)

	got, err := GetPrompt(promptName, nil)
	assert.NoError(t, err)
	assert.Equal(t, overrideContent, got)
}

func TestGetPrompt_Error(t *testing.T) {
	// Non-existent prompt
	got, err := GetPrompt("non-existent-prompt", nil)
	assert.Error(t, err)
	assert.Empty(t, got)
	assert.True(t, strings.Contains(err.Error(), "failed to read prompt template"), "Expected error message")
}
