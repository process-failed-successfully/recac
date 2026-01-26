package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// runbookExecCommand allows mocking in tests
var runbookExecCommand = exec.Command

func promptUser(cmd *cobra.Command, reader *bufio.Reader) (string, error) {
	for {
		fmt.Fprint(cmd.OutOrStdout(), "Execute this block? [y]es, [n]o, [q]uit: ")
		input, err := reader.ReadString('\n')

		cleanInput := strings.TrimSpace(strings.ToLower(input))
		if cleanInput == "y" || cleanInput == "yes" {
			return "y", nil
		}
		if cleanInput == "n" || cleanInput == "no" {
			return "n", nil
		}
		if cleanInput == "q" || cleanInput == "quit" {
			return "q", nil
		}

		if err != nil {
			// EOF or error, default to quit
			return "q", nil
		}
	}
}

func askToContinue(cmd *cobra.Command, reader *bufio.Reader) bool {
	fmt.Fprint(cmd.OutOrStdout(), "Command failed. Continue anyway? [y/N]: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

func executeBlock(code string, env map[string]string, tmpDir string, cmd *cobra.Command) (map[string]string, error) {
	// 1. Prepare command
	// We run: sh -c "<code>; env > env.new"
	// We handle set -e to stop on error within the block
	outFile := filepath.Join(tmpDir, "env_out.txt")
	// Make sure outFile doesn't exist
	os.Remove(outFile)

	// Use sh or bash
	shell := "bash"
	if _, err := exec.LookPath("bash"); err != nil {
		shell = "sh"
	}

	// We wrap the user code to ensure we capture env even if it's multiple lines
	wrappedCode := fmt.Sprintf("set -e\n%s\nenv > '%s'", code, outFile)

	runCmd := runbookExecCommand(shell, "-c", wrappedCode)
	// We stream stdout/stderr
	runCmd.Stdout = cmd.OutOrStdout()
	runCmd.Stderr = cmd.ErrOrStderr()

	var envList []string
	for k, v := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}
	runCmd.Env = envList

	if err := runCmd.Run(); err != nil {
		return nil, err
	}

	// 2. Read new env
	lines, err := readLines(outFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read output env: %w", err)
	}

	newEnv := make(map[string]string)
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			newEnv[parts[0]] = parts[1]
		}
	}

	return newEnv, nil
}
