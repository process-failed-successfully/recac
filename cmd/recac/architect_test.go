package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunArchitectCmd(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "app_spec.txt")
	outDir := filepath.Join(tmpDir, "out")
	err := os.WriteFile(specPath, []byte("Feature: Login"), 0644)
	require.NoError(t, err)

	validArch := `
system_name: MyApp
version: "1.0.0"
components:
  - id: api
    name: API
    type: service
    description: backend
relationships: []
`
	filesJSON := fmt.Sprintf(`{
		"architecture.yaml": %q
	}`, validArch)

	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		mockAgent := agent.NewMockAgent()
		mockAgent.SetResponse(filesJSON)
		return mockAgent, nil
	}

	origExit := exit
	defer func() { exit = origExit }()
	exit = func(code int) {
		panic(fmt.Sprintf("exit-%d", code))
	}

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	architectCmd.Flags().Set("spec", specPath)
	architectCmd.Flags().Set("out", outDir)

	defer func() {
		if p := recover(); p != nil {
			w.Close()
			var buf bytes.Buffer
			buf.ReadFrom(r)
			t.Logf("Output on panic: %s", buf.String())
			t.Errorf("Unexpected panic: %v", p)
		}
	}()

	runArchitectCmd(architectCmd, []string{})

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !assert.Contains(t, output, "SUCCESS: Architecture is valid.") {
		t.Logf("Output: %s", output)
	}

	archFile := filepath.Join(outDir, "architecture.yaml")
	if _, err := os.Stat(archFile); os.IsNotExist(err) {
		t.Logf("File not found: %s", archFile)
		// List tmpDir
		entries, _ := os.ReadDir(tmpDir)
		for _, e := range entries {
			t.Logf(" - %s", e.Name())
			if e.IsDir() && e.Name() == "out" {
				subs, _ := os.ReadDir(filepath.Join(tmpDir, "out"))
				for _, s := range subs {
					t.Logf("   - %s", s.Name())
				}
			}
		}
	}
	assert.FileExists(t, archFile)
}
