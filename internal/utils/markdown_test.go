package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanCodeBlock(t *testing.T) {
	t.Run("Extracts from markdown", func(t *testing.T) {
		markdown := "Here is the code:\n```go\nfunc foo() {}\n```\nHope it helps."
		cleaned := CleanCodeBlock(markdown)
		assert.Equal(t, "func foo() {}", cleaned)
	})

	t.Run("Returns raw if no markdown", func(t *testing.T) {
		raw := "func foo() {}"
		cleaned := CleanCodeBlock(raw)
		assert.Equal(t, "func foo() {}", cleaned)
	})
}

func TestCleanJSONBlock(t *testing.T) {
	t.Run("Extracts from markdown with json tag", func(t *testing.T) {
		markdown := "Here is the json:\n```json\n{\"foo\": \"bar\"}\n```"
		cleaned := CleanJSONBlock(markdown)
		assert.Equal(t, "{\"foo\": \"bar\"}", cleaned)
	})

	t.Run("Extracts from markdown without tag", func(t *testing.T) {
		markdown := "Here is the json:\n```\n{\"foo\": \"bar\"}\n```"
		cleaned := CleanJSONBlock(markdown)
		assert.Equal(t, "{\"foo\": \"bar\"}", cleaned)
	})

	t.Run("Extracts using braces fallback", func(t *testing.T) {
		text := "Sure! {\"foo\": \"bar\"} is the answer."
		cleaned := CleanJSONBlock(text)
		assert.Equal(t, "{\"foo\": \"bar\"}", cleaned)
	})
}
