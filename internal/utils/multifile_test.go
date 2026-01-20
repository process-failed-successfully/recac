package utils

import (
	"reflect"
	"testing"
)

func TestParseFileBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name: "Single file",
			input: `<file path="main.go">
package main
func main() {}
</file>`,
			expected: map[string]string{
				"main.go": "package main\nfunc main() {}\n",
			},
		},
		{
			name: "Multiple files",
			input: `<file path="a.txt">
Hello
</file>
Some text in between
<file path="b.txt">
World
</file>`,
			expected: map[string]string{
				"a.txt": "Hello\n",
				"b.txt": "World\n",
			},
		},
		{
			name: "With extra whitespace",
			input: `
<file path="foo.go">
  code
</file>
`,
			expected: map[string]string{
				"foo.go": "code\n",
			},
		},
		{
			name:     "No files",
			input:    "Just some text",
			expected: map[string]string{},
		},
		{
			name: "Code inside",
			input: `<file path="logic.go">
if a < b {
	return b
}
</file>`,
			expected: map[string]string{
				"logic.go": "if a < b {\n\treturn b\n}\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseFileBlocks(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ParseFileBlocks() = %v, want %v", got, tt.expected)
			}
		})
	}
}
