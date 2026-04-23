package analysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateSequence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simple Go project structure
	// main.go -> pkg.DoSomething() -> pkg.helper()

	mainContent := `package main

import "example.com/project/pkg"

func main() {
	pkg.DoSomething()
}
`
	pkgDir := filepath.Join(tmpDir, "pkg")
	err := os.MkdirAll(pkgDir, 0755)
	require.NoError(t, err)

	pkgContent := `package pkg

func DoSomething() {
	helper()
}

func helper() {}
`

	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(pkgDir, "lib.go"), []byte(pkgContent), 0644)
	require.NoError(t, err)

	// Create go.mod
	goModContent := `module example.com/project

go 1.21
`
	err = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644)
	require.NoError(t, err)

	mermaid, err := GenerateSequence(tmpDir, "main.main", 5)
	require.NoError(t, err)

	t.Logf("Generated Mermaid:\n%s", mermaid)

	require.Contains(t, mermaid, "sequenceDiagram")
	// The implementation groups participants by package (or package.Receiver)
	require.Contains(t, mermaid, "participant main as main")
	require.Contains(t, mermaid, "participant pkg as pkg")

	// Check interactions
	require.Contains(t, mermaid, "main->>pkg: DoSomething()")
	require.Contains(t, mermaid, "pkg->>pkg: helper()")
}

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"pkg.Func", "pkg_Func"},
		{"pkg/sub.Func", "pkg_sub_Func"},
		{"pkg-name.Func", "pkg_name_Func"},
	}

	for _, tt := range tests {
		got := sanitizeID(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestGenerateSequence_Errors(t *testing.T) {
	tmpDir := t.TempDir()

	// Test missing entry point
	_, err := GenerateSequence(tmpDir, "nonexistent.main", 5)
	require.Error(t, err)
	require.Contains(t, err.Error(), "entry point 'nonexistent.main' not found")

	// Test ambiguous entry point
	file1 := `package pkg
func Ambiguous() {}
`
	file2 := `package other
func Ambiguous() {}
`
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "pkg"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "pkg", "f1.go"), []byte(file1), 0644))

	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "other"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "other", "f2.go"), []byte(file2), 0644))

	_, err = GenerateSequence(tmpDir, "Ambiguous", 5)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ambiguous entry point 'Ambiguous'")
}

func TestGenerateSequence_Receiver(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package main

type MyStruct struct{}

func (m *MyStruct) Method() {}

func main() {
	var m MyStruct
	m.Method()
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0644))

	mermaid, err := GenerateSequence(tmpDir, "main.main", 5)
	require.NoError(t, err)
	require.Contains(t, mermaid, "participant main as main")
	// main->>main.MyStruct: Method()
}
