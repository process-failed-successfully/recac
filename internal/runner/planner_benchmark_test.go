package runner

import (
	"testing"
)

var result string

func BenchmarkCleanJSON_WithMarkdown(b *testing.B) {
	input := "```json\n{\"foo\": \"bar\"}\n```"
	var r string
	for i := 0; i < b.N; i++ {
		r = cleanJSON(input)
	}
	result = r
}

func BenchmarkCleanJSON_NoMarkdown(b *testing.B) {
	input := "{\"foo\": \"bar\"}"
	var r string
	for i := 0; i < b.N; i++ {
		r = cleanJSON(input)
	}
	result = r
}

func BenchmarkCleanJSON_GenericMarkdown(b *testing.B) {
	input := "```\n{\"foo\": \"bar\"}\n```"
	var r string
	for i := 0; i < b.N; i++ {
		r = cleanJSON(input)
	}
	result = r
}
