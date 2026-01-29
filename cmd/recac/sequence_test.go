package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSequenceCmd(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "recac-sequence-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 2. Create Sample Code
	// main.go
	mainContent := `package main

func main() {
	Start()
}

func Start() {
	Worker()
	Helper()
}

func Worker() {
	Helper()
}

func Helper() {}
`
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// 3. Prepare Command Environment
	// Save original state
	originalDir := sequenceDir
	originalDepth := sequenceDepth
	defer func() {
		sequenceDir = originalDir
		sequenceDepth = originalDepth
	}()

	// Set test state
	sequenceDir = tmpDir
	sequenceDepth = 5

	// 4. Run Command
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Run targeting "main.Start"
	// Note: in our simplified logic, main package in root dir is just "main"
	// So IDs are "main.Start", "main.Worker", etc.
	err = runSequence(cmd, []string{"main.Start"})
	require.NoError(t, err)

	output := buf.String()
	t.Logf("Output:\n%s", output)

	// 5. Assertions
	assert.Contains(t, output, "sequenceDiagram")
	assert.Contains(t, output, "User->>main_Start: Start()")

	// Check connections
	assert.Contains(t, output, "main_Start->>main_Worker: Worker()")
	assert.Contains(t, output, "main_Start->>main_Helper: Helper()")
	assert.Contains(t, output, "main_Worker->>main_Helper: Helper()")

	// Check Order: Start calls Worker BEFORE Helper
	idxWorker := strings.Index(output, "main_Start->>main_Worker")
	idxHelper := strings.Index(output, "main_Start->>main_Helper")

	assert.Greater(t, idxWorker, 0, "Should find call to Worker")
	assert.Greater(t, idxHelper, 0, "Should find call to Helper")
	assert.Less(t, idxWorker, idxHelper, "Start should call Worker before Helper")
}

func TestSequenceCmd_Recursion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-sequence-recursion")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	content := `package main

func A() {
	B()
}

func B() {
	A() // Recursion
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "loop.go"), []byte(content), 0644)
	require.NoError(t, err)

	// Setup
	originalDir := sequenceDir
	defer func() { sequenceDir = originalDir }()
	sequenceDir = tmpDir
	sequenceDepth = 5

	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err = runSequence(cmd, []string{"main.A"})
	require.NoError(t, err)

	output := buf.String()

	// Should see A -> B
	assert.Contains(t, output, "main_A->>main_B: B()")
	// Should see B -> A (Recursive)
	assert.Contains(t, output, "main_B->>main_A: A() (Recursive)")
	// Should NOT see another A -> B (loop broken)
	// We count occurrences
	count := strings.Count(output, "main_A->>main_B")
	assert.Equal(t, 1, count, "Should only call A->B once")
}
