package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestParseRunbook(t *testing.T) {
	content := `
# Title

Some text.

` + "```bash" + `
echo "hello"
` + "```" + `

More text.

` + "```sh" + `
export VAR=1
` + "```" + `
`
	tmpFile := filepath.Join(t.TempDir(), "test.md")
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	assert.NoError(t, err)

	blocks, err := parseRunbook(tmpFile)
	assert.NoError(t, err)
	assert.Len(t, blocks, 4) // text, bash, text, sh

	assert.Equal(t, "text", blocks[0].Type)
	assert.Contains(t, blocks[0].Content, "# Title")

	assert.Equal(t, "code", blocks[1].Type)
	assert.Equal(t, "bash", blocks[1].Lang)
	assert.Contains(t, blocks[1].Content, "echo \"hello\"")

	assert.Equal(t, "text", blocks[2].Type)

	assert.Equal(t, "code", blocks[3].Type)
	assert.Equal(t, "sh", blocks[3].Lang)
	assert.Contains(t, blocks[3].Content, "export VAR=1")
}

func TestExecuteBlock(t *testing.T) {
	tmpDir := t.TempDir()
	code := "echo test"

	// Mock exec
	runbookExecCommand = func(name string, arg ...string) *exec.Cmd {
		// We use double dash to stop flag parsing in the test binary
		cs := []string{"-test.run=TestRunbookHelperProcess", "--", "--", name}
		cs = append(cs, arg...)
		exe, _ := os.Executable()
		cmd := exec.Command(exe, cs...)
		// Env will be overwritten by runbook.go, so we rely on args
		return cmd
	}
	defer func() { runbookExecCommand = exec.Command }()

	// Prepare env with flags if needed, but here we use Args passing
	env := map[string]string{
		"INITIAL":                "val",
		"GO_WANT_HELPER_PROCESS": "1",
	}

	// Create dummy command
	dummyCmd := &cobra.Command{}
	var outBuf bytes.Buffer
	dummyCmd.SetOut(&outBuf)
	dummyCmd.SetErr(&outBuf)

	// We pass 'code' which will be wrapped.
	// The helper should see the wrapped code.
	newEnv, err := executeBlock(code, env, tmpDir, dummyCmd)
	if err != nil {
		t.Logf("Output: %s", outBuf.String())
	}
	assert.NoError(t, err)

	// Our helper process writes MOCK_KEY=MOCK_VAL to the output file
	assert.Equal(t, "MOCK_VAL", newEnv["MOCK_KEY"])
	assert.Equal(t, "val", newEnv["INITIAL"])
}

func TestRunbookHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Println("DEBUG: Helper started")

	// Parse args to find where actual command starts
	// We look for "--" then another "--"?
	// Or just look for "bash" or "sh"?
	// Or just skip until we find "--" then next is what we want?

	args := os.Args
	startIdx := -1
	for i, arg := range args {
		if arg == "--" {
			// Found the first -- separator
			// If next is also --, assume we used double separator strategy
			if i+1 < len(args) && args[i+1] == "--" {
				startIdx = i + 2
			} else {
				startIdx = i + 1
			}
			break
		}
	}

	if startIdx == -1 || startIdx >= len(args) {
		fmt.Printf("Could not find arguments after separator: %v\n", args)
		os.Exit(1)
	}

	cmdArgs := args[startIdx:]
	if len(cmdArgs) < 3 {
		fmt.Printf("Not enough command args: %v\n", cmdArgs)
		os.Exit(1)
	}

	// cmdArgs[0] = bash/sh
	// cmdArgs[1] = -c
	// cmdArgs[2] = wrappedCode

	code := cmdArgs[2]
	fmt.Printf("Helper received code: %s\n", code)
	// We extract the filename.
	re := regexp.MustCompile(`env > '([^']+)'`)
	matches := re.FindStringSubmatch(code)
	if len(matches) < 2 {
		fmt.Fprintf(os.Stderr, "Could not find output file in code: %s\n", code)
		os.Exit(1)
	}
	outFile := matches[1]

	// Write mock env
	content := "MOCK_KEY=MOCK_VAL\nINITIAL=val\n"
	err := os.WriteFile(outFile, []byte(content), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write to %s: %v\n", outFile, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Helper wrote to %s\n", outFile)

	os.Exit(0)
}
