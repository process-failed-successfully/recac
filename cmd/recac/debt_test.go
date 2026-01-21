package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDebtCmd(t *testing.T) {
	// 1. Setup temp dir with a file containing TODO
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")
	content := `package main
// TODO: Refactor this mess
func main() {}
`
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// 2. Mock execCommand
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "git" && len(arg) > 0 && arg[0] == "blame" {
			// Call the helper process
			exe, _ := os.Executable()
			cmd := exec.Command(exe, "-test.run=TestDebtHelperProcess", "--", "blame")
			cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
			return cmd
		}
		return origExec(name, arg...)
	}

	// 3. Run command
	cmd := debtCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Reset flags
	debtJSON = false
	debtMinAge = ""
	debtAuthor = ""

	err = runDebt(cmd, []string{tempDir})

	// If git is not installed, it might fail. We should check that error or skip.
	if err != nil && err.Error() == "git is not installed or not in PATH" {
		t.Skip("git not installed")
	}
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Refactor this mess")
	assert.Contains(t, output, "John Doe")
	// Since we mock 2 years ago
	assert.Contains(t, output, "2.0y")
}

func TestDebtCmd_FilterAge(t *testing.T) {
	// 1. Setup temp dir with a file containing TODO
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")
	content := `package main
// TODO: Refactor this mess
func main() {}
`
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// 2. Mock execCommand
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "git" && len(arg) > 0 && arg[0] == "blame" {
			exe, _ := os.Executable()
			cmd := exec.Command(exe, "-test.run=TestDebtHelperProcess", "--", "blame")
			cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
			return cmd
		}
		return origExec(name, arg...)
	}

	// 3. Run command with min-age 3y (should filter out our 2y old item)
	cmd := debtCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Reset flags
	debtJSON = false
	debtMinAge = "3y"
	debtAuthor = ""

	err = runDebt(cmd, []string{tempDir})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No matching technical debt found")
}

func TestDebtCmd_JSON(t *testing.T) {
	// 1. Setup temp dir with a file containing TODO
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")
	content := `package main
// TODO: Refactor this mess
func main() {}
`
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// 2. Mock execCommand
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "git" && len(arg) > 0 && arg[0] == "blame" {
			exe, _ := os.Executable()
			cmd := exec.Command(exe, "-test.run=TestDebtHelperProcess", "--", "blame")
			cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
			return cmd
		}
		return origExec(name, arg...)
	}

	// 3. Run command
	cmd := debtCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Reset flags
	debtJSON = true
	debtMinAge = ""
	debtAuthor = ""

	err = runDebt(cmd, []string{tempDir})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, `"author": "John Doe"`)
	assert.Contains(t, output, `"age": "2.0y"`)
}

// TestDebtHelperProcess is the mock git blame
func TestDebtHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		return
	}

	if args[0] == "blame" {
		// Output porcelain format
		// timestamp 2 years ago
		twoYearsAgo := time.Now().AddDate(-2, 0, 0).Unix()

		fmt.Printf("4e5d6f7a 1 1 1\n")
		fmt.Printf("author John Doe\n")
		fmt.Printf("author-time %d\n", twoYearsAgo)
		os.Exit(0)
	}
	os.Exit(1)
}
