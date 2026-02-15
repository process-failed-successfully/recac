package utils

import (
	"strings"
	"testing"
)

func BenchmarkCleanJSONBlock_Simple(b *testing.B) {
	input := `{"key": "value"}`
	for i := 0; i < b.N; i++ {
		CleanJSONBlock(input)
	}
}

func BenchmarkCleanJSONBlock_MarkdownJSON(b *testing.B) {
	input := "```json\n{\"key\": \"value\", \"nested\": {\"a\": 1}}\n```"
	for i := 0; i < b.N; i++ {
		CleanJSONBlock(input)
	}
}

func BenchmarkCleanJSONBlock_LargeMarkdown(b *testing.B) {
	// Simulate a large LLM response
	// The strings.Repeat uses backticks, so quotes inside are fine.
	repeated := strings.Repeat(`{"id": 123, "name": "test", "data": "some long string data"},`, 100)
	input := "Here is the JSON you requested:\n\n```json\n" +
		repeated +
		"{\"id\": 124, \"name\": \"last\"}\n```\n\nHope this helps!"
	for i := 0; i < b.N; i++ {
		CleanJSONBlock(input)
	}
}

func BenchmarkCleanJSONBlock_Fallback(b *testing.B) {
	input := "Here is the JSON: {\"key\": \"value\"} Thanks."
	for i := 0; i < b.N; i++ {
		CleanJSONBlock(input)
	}
}
