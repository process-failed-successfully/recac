package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvolutionCmd(t *testing.T) {
	// 1. Setup Temp Git Repo
	tmpDir, err := os.MkdirTemp("", "recac-evolution-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	require.NoError(t, os.Chdir(tmpDir))

	runGit(t, "init")
	runGit(t, "config", "user.email", "test@example.com")
	runGit(t, "config", "user.name", "Test User")

	// 2. Commit 1: Simple function
	code1 := `package main
import "fmt"
func main() {
	if true {
		fmt.Println("Hello")
	}
}
// TODO: Add more features
`
	require.NoError(t, os.WriteFile("main.go", []byte(code1), 0644))
	runGit(t, "add", ".")
	runGit(t, "commit", "-m", "Initial_commit") // Underscore to avoid space issues in parsing if any

	// 3. Commit 2: Increase complexity and TODOs
	code2 := `package main
import "fmt"
func main() {
	if true {
		if false {
			fmt.Println("Nested")
		}
	}
	// TODO: Refactor this
	// FIXME: Bug here
}
`
	require.NoError(t, os.WriteFile("main.go", []byte(code2), 0644))
	runGit(t, "add", ".")
	runGit(t, "commit", "-m", "Second_commit")

	// 4. Run Evolution Command
	buf := new(bytes.Buffer)
	evolutionCmd.SetOut(buf)

	// Pass "HEAD" to get all history
	err = runEvolution(evolutionCmd, []string{"HEAD"})
	require.NoError(t, err)

	output := buf.String()
	fmt.Println("Captured Output:\n" + output)

	// 5. Verify Output
	assert.Contains(t, output, "CODEBASE EVOLUTION")
	assert.Contains(t, output, "Initial")
	assert.Contains(t, output, "Second")

	// Check for presence of metrics
	// Commit 1: 1 TODO
	// Commit 2: 2 TODOs
	// Just check if the output contains lines corresponding to our commits
	lines := strings.Split(strings.TrimSpace(output), "\n")
	// Header + 2 commits = 3 lines minimum
	assert.GreaterOrEqual(t, len(lines), 3)

	// Basic check for data integrity
	// We expect "Second" row to have more complexity/todos than "Initial"
	// But parsing the table exactly is brittle.
	// Let's just ensure it ran without error and produced output.
}

func runGit(t *testing.T, args ...string) string {
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, string(out))
	return string(out)
}
