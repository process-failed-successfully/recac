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

func TestParseMarkdownBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []MarkdownBlock
	}{
		{
			name:     "Empty",
			input:    []string{},
			expected: nil,
		},
		{
			name:     "Text Only",
			input:    []string{"Hello", "World"},
			expected: []MarkdownBlock{{Type: "text", Content: "Hello\nWorld\n"}},
		},
		{
			name:     "Code Only",
			input:    []string{"```go", "package main", "```"},
			expected: []MarkdownBlock{{Type: "code", Content: "package main\n", Lang: "go"}},
		},
		{
			name:  "Mixed",
			input: []string{"Intro", "```bash", "echo hi", "```", "Outro"},
			expected: []MarkdownBlock{
				{Type: "text", Content: "Intro\n"},
				{Type: "code", Content: "echo hi\n", Lang: "bash"},
				{Type: "text", Content: "Outro\n"},
			},
		},
		{
			name:  "Unclosed Code",
			input: []string{"Intro", "```python", "print('hi')"},
			expected: []MarkdownBlock{
				{Type: "text", Content: "Intro\n"},
				{Type: "code", Content: "print('hi')\n", Lang: "python"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseMarkdownBlocks(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("len(got) = %d, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("got[%d] = %+v, want %+v", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestExtractCodeBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []CodeBlock
	}{
		{
			name:     "No code block",
			input:    "Just text",
			expected: nil,
		},
		{
			name:     "Single block",
			input:    "Here is code:\n```go\nfmt.Println(\"Hi\")\n```",
			expected: []CodeBlock{{Language: "go", Content: "fmt.Println(\"Hi\")"}},
		},
		{
			name:  "Multiple blocks",
			input: "Block 1:\n```bash\necho 1\n```\nBlock 2:\n```python\nprint(2)\n```",
			expected: []CodeBlock{
				{Language: "bash", Content: "echo 1"},
				{Language: "python", Content: "print(2)"},
			},
		},
		{
			name:     "No language",
			input:    "```\nplain text\n```",
			expected: []CodeBlock{{Language: "", Content: "plain text"}},
		},
		{
			name:     "Indented block (fence)",
			input:    "  ```xml\n  <tag></tag>\n  ```",
			expected: []CodeBlock{{Language: "xml", Content: "<tag></tag>"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractCodeBlocks(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("len(got) = %d, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("got[%d] = %+v, want %+v", i, got[i], tt.expected[i])
				}
			}
		})
	}
}
