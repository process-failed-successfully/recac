package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditCmd(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "audit-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 2. Create Test Files

	// Good file
	err = os.WriteFile(filepath.Join(tmpDir, "good.go"), []byte(`
package main
import "fmt"
func hello() {
    fmt.Println("Hello")
}
`), 0644)
	require.NoError(t, err)

	// Bad file (High Complexity)
	// 16 branches to ensure complexity > 15
	badContent := `
package main
func complexFunc() {
    i := 0
    if i == 0 { }
    if i == 1 { }
    if i == 2 { }
    if i == 3 { }
    if i == 4 { }
    if i == 5 { }
    if i == 6 { }
    if i == 7 { }
    if i == 8 { }
    if i == 9 { }
    if i == 10 { }
    if i == 11 { }
    if i == 12 { }
    if i == 13 { }
    if i == 14 { }
    if i == 15 { }
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "bad.go"), []byte(badContent), 0644)
	require.NoError(t, err)

	// Duplicated files
	// Need at least 10 lines (default min-lines)
	dupContent := `
package main
import "fmt"
func duplicate() {
    fmt.Println("This is a duplicated block of code.")
    fmt.Println("It has enough lines to trigger CPD.")
    fmt.Println("Line 3")
    fmt.Println("Line 4")
    fmt.Println("Line 5")
    fmt.Println("Line 6")
    fmt.Println("Line 7")
    fmt.Println("Line 8")
    fmt.Println("Line 9")
    fmt.Println("Line 10")
    fmt.Println("Line 11")
    fmt.Println("Line 12")
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "dup1.go"), []byte(dupContent), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "dup2.go"), []byte(dupContent), 0644)
	require.NoError(t, err)

	// TODOs
	todoContent := `
package main
// TODO: Fix this later
// FIXME: This is broken
`
	err = os.WriteFile(filepath.Join(tmpDir, "todo.go"), []byte(todoContent), 0644)
	require.NoError(t, err)

	// 3. Run Audit (Text Output)
	// Reset globals if needed (though executeCommand resets flags, it doesn't reset vars unless they are bound to flags)
	// auditPath is bound to --path flag.

	output, err := executeCommand(rootCmd, "audit", tmpDir)
	require.NoError(t, err)

	// 4. Verify Output
	assert.Contains(t, output, "AUDIT REPORT")
	assert.Contains(t, output, "COMPLEXITY")
	assert.Contains(t, output, "DUPLICATION")
	assert.Contains(t, output, "MAINTENANCE")

	// Check for specific findings
	// Complexity: complexFunc should be high risk
	assert.Contains(t, output, "complexFunc")

	// Duplication: Should find blocks
	assert.Contains(t, output, "Blocks")
	assert.Contains(t, output, "1")

	// TODOs: Should find 2
	assert.Contains(t, output, "TODOs")
	assert.Contains(t, output, "2")

	// 5. Verify JSON Output
	outputJson, err := executeCommand(rootCmd, "audit", tmpDir, "--json")
	require.NoError(t, err)
	assert.Contains(t, outputJson, `"score":`)
	assert.Contains(t, outputJson, `"complexity":`)
	assert.Contains(t, outputJson, `"duplication":`)
	assert.Contains(t, outputJson, `"todos":`)
	assert.Contains(t, outputJson, `"count": 2`)

	// 6. Verify Fail Flag
	// We expect the score to be somewhat low due to penalties.
	// 100 - 2 (complex) - 2 (dup) - 0.2 (todo) = ~95.8 -> 96
	// Wait, penalties:
	// complex: -2 * 1 = -2
	// dup: -2 * 1 = -2
	// todo: 0.1 * 2 = 0.2
	// Score ~ 96.
	// So let's set min-score to 99 to force failure.

	_, err = executeCommand(rootCmd, "audit", tmpDir, "--min-score", "99", "--fail")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "audit failed")
}
