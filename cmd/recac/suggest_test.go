package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGeneratePrompt(t *testing.T) {
	sType := "refactor"
	limit := 5
	context := "package main\nfunc main() {}"

	prompt := generatePrompt(sType, limit, context)

	assert.Contains(t, prompt, "Focus on: refactor")
	assert.Contains(t, prompt, "list up to 5 high-value suggestions")
	assert.Contains(t, prompt, context)
}

func TestParseSuggestions(t *testing.T) {
	jsonResp := `
[
  {
    "title": "Fix bug",
    "description": "It is broken",
    "type": "bug",
    "file": "main.go"
  }
]
`
	suggestions, err := parseSuggestions(jsonResp)
	assert.NoError(t, err)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "Fix bug", suggestions[0].Title)
	assert.Equal(t, "bug", suggestions[0].Type)
}

func TestParseSuggestions_InvalidJSON(t *testing.T) {
	jsonResp := `invalid json`
	_, err := parseSuggestions(jsonResp)
	assert.Error(t, err)
}

func TestParseSuggestions_MarkdownWrapped(t *testing.T) {
	jsonResp := "```json\n[{\"title\": \"A\"}]\n```"
	suggestions, err := parseSuggestions(jsonResp)
	assert.NoError(t, err)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "A", suggestions[0].Title)
}
