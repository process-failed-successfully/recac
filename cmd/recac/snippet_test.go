package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

type SnippetMockAgent struct {
	Response string
}

func (m *SnippetMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *SnippetMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestSnippetCmd(t *testing.T) {
	// Setup temp home for global snippets
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome) // Unix
	t.Setenv("USERPROFILE", tmpHome) // Windows

	// Setup temp CWD for local snippets and target files
	tmpCwd := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(tmpCwd); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}

	// Mock Agent
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := &SnippetMockAgent{Response: "Adapted Content"}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	t.Run("Add Snippet (Stdin)", func(t *testing.T) {
		// Reset command to ensure clean state
		cmd := &cobra.Command{}
		// We need to use the run function directly or setup the command properly
		// Reusing the global variable might be risky if flags are involved, but for add it's fine.
		// However, SetIn/SetOut modifies the command struct.
		// Let's create a fresh command for testing if possible, or reset the global one.
		// Since snippetAddCmd is a global variable, modifying it persists across tests.
		// Better to use a fresh struct if logic allows.
		// runSnippetAdd takes *cobra.Command.

		cmd.SetOut(new(bytes.Buffer))
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		// Create a pipe to simulate stdin
		r, w, _ := os.Pipe()
		cmd.SetIn(r)

		go func() {
			w.Write([]byte("fmt.Println(\"Hello\")"))
			w.Close()
		}()

		err := runSnippetAdd(cmd, []string{"hello.go"})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "saved to")

		// Verify file exists in global or local
		// Since no .recac in CWD, should be in HOME/.recac/snippets
		expectedPath := filepath.Join(tmpHome, ".recac", "snippets", "hello.go")
		content, err := os.ReadFile(expectedPath)
		assert.NoError(t, err)
		assert.Equal(t, "fmt.Println(\"Hello\")", string(content))
	})

	t.Run("List Snippets", func(t *testing.T) {
		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := runSnippetList(cmd, []string{})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "hello.go")
	})

	t.Run("Show Snippet", func(t *testing.T) {
		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := runSnippetShow(cmd, []string{"hello.go"})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "fmt.Println(\"Hello\")")
	})

	t.Run("Apply Snippet (Stdout)", func(t *testing.T) {
		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := runSnippetApply(cmd, []string{"hello.go"})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "fmt.Println(\"Hello\")")
	})

	t.Run("Apply Snippet (File)", func(t *testing.T) {
		targetFile := "target.go"
		os.WriteFile(targetFile, []byte("package main\n\nfunc main() {\n"), 0644)

		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := runSnippetApply(cmd, []string{"hello.go", targetFile})
		assert.NoError(t, err)

		content, _ := os.ReadFile(targetFile)
		assert.Contains(t, string(content), "fmt.Println(\"Hello\")")
	})

	t.Run("Apply Snippet with AI", func(t *testing.T) {
		snippetApplyAI = true
		defer func() { snippetApplyAI = false }()

		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(new(bytes.Buffer))

		err := runSnippetApply(cmd, []string{"hello.go"})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Adapted Content")
	})

	t.Run("Remove Snippet", func(t *testing.T) {
		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := runSnippetRemove(cmd, []string{"hello.go"})
		assert.NoError(t, err)

		// Verify deletion
		expectedPath := filepath.Join(tmpHome, ".recac", "snippets", "hello.go")
		_, err = os.Stat(expectedPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("Invalid Snippet Name", func(t *testing.T) {
		cmd := &cobra.Command{}
		err := runSnippetAdd(cmd, []string{"../bad.go"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid snippet name")
	})
}
