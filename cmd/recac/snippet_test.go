package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/cmdutils"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnippetCommands(t *testing.T) {
	tempDir := t.TempDir()
	localDir := filepath.Join(tempDir, ".recac", "snippets")
	globalDir := filepath.Join(tempDir, "home", ".recac", "snippets")

	// Mock getSnippetDirs
	oldGetSnippetDirs := getSnippetDirs
	getSnippetDirs = func() []string {
		return []string{localDir, globalDir}
	}
	defer func() { getSnippetDirs = oldGetSnippetDirs }()

	// Reset flags
	snippetContent = ""
	snippetUseAI = false

	t.Run("Add Snippet", func(t *testing.T) {
		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		snippetContent = "fmt.Println(\"Hello\")"
		err := runSnippetAdd(cmd, []string{"hello"})
		require.NoError(t, err)

		assert.Contains(t, buf.String(), "added to")
		assert.FileExists(t, filepath.Join(localDir, "hello"))

		content, err := os.ReadFile(filepath.Join(localDir, "hello"))
		require.NoError(t, err)
		assert.Equal(t, "fmt.Println(\"Hello\")", string(content))
	})

	t.Run("List Snippets", func(t *testing.T) {
		// Create a global snippet manually
		err := os.MkdirAll(globalDir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(globalDir, "global-hello"), []byte("echo hello"), 0644)
		require.NoError(t, err)

		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)

		err = runSnippetList(cmd, []string{})
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "hello")
		assert.Contains(t, output, "global-hello")
		assert.Contains(t, output, localDir)
		assert.Contains(t, output, globalDir)
	})

	t.Run("Apply Snippet", func(t *testing.T) {
		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		snippetUseAI = false
		err := runSnippetApply(cmd, []string{"hello"})
		require.NoError(t, err)

		assert.Equal(t, "fmt.Println(\"Hello\")", buf.String())
	})

	t.Run("Delete Snippet", func(t *testing.T) {
		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := runSnippetDelete(cmd, []string{"hello"})
		require.NoError(t, err)

		assert.Contains(t, buf.String(), "deleted from")
		assert.NoFileExists(t, filepath.Join(localDir, "hello"))
	})
}

func TestSnippetApplyAI(t *testing.T) {
	tempDir := t.TempDir()
	localDir := filepath.Join(tempDir, ".recac", "snippets")

	oldGetSnippetDirs := getSnippetDirs
	getSnippetDirs = func() []string {
		return []string{localDir}
	}
	defer func() { getSnippetDirs = oldGetSnippetDirs }()

	// Create a snippet with placeholders
	err := os.MkdirAll(localDir, 0755)
	require.NoError(t, err)
	snippetName := "greeting"
	snippetBody := `func main() { fmt.Printf("Hello, {{ name }}!") }`
	err = os.WriteFile(filepath.Join(localDir, snippetName), []byte(snippetBody), 0644)
	require.NoError(t, err)

	// Mock Agent
	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse("```go\nfunc main() { fmt.Printf(\"Hello, World!\") }\n```")

	// Mock GetAgentClient
	oldGetAgentClient := cmdutils.GetAgentClient
	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { cmdutils.GetAgentClient = oldGetAgentClient }()

	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	snippetUseAI = true
	err = runSnippetApply(cmd, []string{snippetName})
	require.NoError(t, err)

	assert.Equal(t, "func main() { fmt.Printf(\"Hello, World!\") }", buf.String())
}
