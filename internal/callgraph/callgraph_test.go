package callgraph

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallGraph(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "callgraph_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create sample go file
	code := `package main

func A() {
	B()
}

func B() {
	C()
}

func C() {}

type MyStruct struct{}

func (s *MyStruct) Method() {
	A()
}

func Caller() {
	s := &MyStruct{}
	s.Method()
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(code), 0644)
	require.NoError(t, err)

	// Test 1: Full Graph
	calls, err := AnalyzeCalls(tmpDir)
	require.NoError(t, err)

	output := GenerateMermaidCallGraph(calls, nil, 0)
	t.Logf("Full Graph Output:\n%s", output)

	assert.Contains(t, output, "main_A --> main_B")
	assert.Contains(t, output, "main_B --> main_C")
	// For method call, likely "s.Method" in AST without type info, or "Method"
	// From code logic: resolveCallee -> getTypeName(X) + "." + Sel.Name
	// For s.Method(): X is 's' (Ident). getTypeName('s') = 's'. Output: 's.Method'.
	// So we expect:
	// main_Caller --> s_Method
	// And MyStruct.Method calls A.
	// main_ptr_MyStruct_Method --> main_A

	assert.Contains(t, output, "main_ptr_MyStruct_Method --> main_A")

	// Test 2: Focus on B
	focusRe := regexp.MustCompile("B")
	output = GenerateMermaidCallGraph(calls, focusRe, 0)
	t.Logf("Focus B Output:\n%s", output)

	assert.Contains(t, output, "main_B --> main_C")
	assert.Contains(t, output, "main_A --> main_B")

	// Test 3: Depth Limit
	output = GenerateMermaidCallGraph(calls, focusRe, 1)
	t.Logf("Depth 1 Output:\n%s", output)

	assert.Contains(t, output, "main_B --> main_C")
	assert.Contains(t, output, "main_A --> main_B")

	// Should NOT contain whatever called A (MyStruct.Method)
	// main_ptr_MyStruct_Method -> main_A should be absent
	assert.NotContains(t, output, "Method --> main_A")
}
