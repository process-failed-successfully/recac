package main

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSystemCmd(t *testing.T) {
	// Mock execLookPath
	originalLookPath := execLookPath
	execLookPath = func(file string) (string, error) {
		if file == "orchestrator" {
			return "/usr/local/bin/orchestrator", nil
		}
		if file == "recac-agent" {
			return "", errors.New("executable file not found in $PATH")
		}
		if file == "docker" {
			return "/usr/bin/docker", nil
		}
		if file == "git" {
			return "/usr/bin/git", nil
		}
		return exec.LookPath(file)
	}
	defer func() {
		execLookPath = originalLookPath
	}()

	output, err := executeCommand(rootCmd, "system")
	require.NoError(t, err)

	require.Contains(t, output, "System Diagnostics:")
	require.Contains(t, output, "OS:")
	require.Contains(t, output, "Architecture:")
	require.Contains(t, output, "Go Version:")

	require.Contains(t, output, "Binary Checks:")

	// Split by newline and check lines to be more robust
	lines := strings.Split(output, "\n")

	foundOrchestrator := false
	foundRecacAgent := false
	foundDocker := false
	foundGit := false

	for _, line := range lines {
		if strings.Contains(line, "orchestrator: Found at /usr/local/bin/orchestrator") {
			foundOrchestrator = true
		}
		if strings.Contains(line, "recac-agent: Not Found in PATH") {
			foundRecacAgent = true
		}
		if strings.Contains(line, "docker: Found at /usr/bin/docker") {
			foundDocker = true
		}
		if strings.Contains(line, "git: Found at /usr/bin/git") {
			foundGit = true
		}
	}

	require.True(t, foundOrchestrator, "Expected orchestrator to be found at /usr/local/bin/orchestrator")
	require.True(t, foundRecacAgent, "Expected recac-agent to be not found")
	require.True(t, foundDocker, "Expected docker to be found at /usr/bin/docker")
	require.True(t, foundGit, "Expected git to be found at /usr/bin/git")
}
