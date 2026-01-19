package utils

import (
	"testing"
)

func TestCleanCodeBlock(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "No code block",
			input:    "fmt.Println(\"Hello\")",
			expected: "fmt.Println(\"Hello\")",
		},
		{
			name:     "With code block",
			input:    "Here is the code:\n```go\nfmt.Println(\"Hello\")\n```",
			expected: "fmt.Println(\"Hello\")",
		},
		{
			name:     "With json block",
			input:    "```json\n{\"foo\": \"bar\"}\n```",
			expected: "{\"foo\": \"bar\"}",
		},
		{
			name:     "Multiple blocks returns first",
			input:    "```go\nBlock 1\n```\n```go\nBlock 2\n```",
			expected: "Block 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanCodeBlock(tt.input); got != tt.expected {
				t.Errorf("CleanCodeBlock() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCleanJSONBlock(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Raw JSON",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "Markdown JSON",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "Markdown without json tag",
			input:    "```\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "Wrapped in text",
			input:    "Here is the JSON:\n{\"key\": \"value\"}\nThanks.",
			expected: `{"key": "value"}`,
		},
		{
			name:     "Array wrapped in text",
			input:    "Here is the list:\n[\"a\", \"b\"]\nThanks.",
			expected: `["a", "b"]`,
		},
		{
			name:     "Empty",
			input:    "",
			expected: "",
		},
		{
			name:     "Markdown with non-json tag",
			input:    "```yaml\nkey: value\n```",
			expected: "key: value",
		},
		{
			name:     "Markdown with json prefix but no braces",
			input:    "```\njson\n\"just a string\"\n```",
			expected: "\"just a string\"",
		},
		{
			name:     "Markdown with json prefix single line",
			input:    "```json \"foo\"```",
			expected: "\"foo\"",
		},
		{
			name:     "Nested braces",
			input:    `{"a": {"b": "c"}}`,
			expected: `{"a": {"b": "c"}}`,
		},
		{
			name:     "Wrapped braces",
			input:    `pre { "a": "b" } post`,
			expected: `{ "a": "b" }`,
		},
		{
			name:     "Unbalanced braces (no end)",
			input:    `pre { "a": "b" post`,
			expected: `pre { "a": "b" post`,
		},
		{
			name:     "Mixed braces start with object",
			input:    `pre { [ ] } post`,
			expected: `{ [ ] }`,
		},
		{
			name:     "Mixed braces start with array",
			input:    `pre [ { } ] post`,
			expected: `[ { } ]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanJSONBlock(tt.input); got != tt.expected {
				t.Errorf("CleanJSONBlock() = %v, want %v", got, tt.expected)
			}
		})
	}
}
