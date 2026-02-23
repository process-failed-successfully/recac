package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRunbook(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "runbook.md")
	content := `
# Header
Some text.

` + "```bash" + `
echo hello
` + "```" + `

More text.
`
	os.WriteFile(file, []byte(content), 0644)

	blocks, err := parseRunbook(file)
	if err != nil {
		t.Fatalf("parseRunbook failed: %v", err)
	}

	if len(blocks) != 3 { // Text, Code, Text
		t.Errorf("Expected 3 blocks, got %d", len(blocks))
	}
	if blocks[1].Type != "code" || blocks[1].Lang != "bash" {
		t.Error("Expected code block")
	}
	if strings.TrimSpace(blocks[1].Content) != "echo hello" {
		t.Error("Expected content 'echo hello'")
	}
}
