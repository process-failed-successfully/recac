package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockArchitectAgent struct {
	Response string
	Err      error
}

func (m *MockArchitectAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	return m.Response, nil
}

func (m *MockArchitectAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestGenerateArchitecture(t *testing.T) {
	t.Run("Valid JSON Response", func(t *testing.T) {
		jsonResp := `
Here is the architecture:
` + "```json" + `
{
	"architecture.yaml": "components: []",
	"docs/api.md": "# API"
}
` + "```"

		mockAgent := &MockArchitectAgent{Response: jsonResp}
		files, err := generateArchitecture(context.Background(), mockAgent, "App Spec")
		require.NoError(t, err)
		assert.Len(t, files, 2)
		assert.Equal(t, "components: []", files["architecture.yaml"])
		assert.Equal(t, "# API", files["docs/api.md"])
	})

	t.Run("Invalid JSON Response", func(t *testing.T) {
		mockAgent := &MockArchitectAgent{Response: "Not JSON"}
		_, err := generateArchitecture(context.Background(), mockAgent, "App Spec")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "json parse error")
	})

	t.Run("Agent Error", func(t *testing.T) {
		mockAgent := &MockArchitectAgent{Err: assert.AnError}
		_, err := generateArchitecture(context.Background(), mockAgent, "App Spec")
		assert.Error(t, err)
		assert.Equal(t, assert.AnError, err)
	})
}

func TestBasePathFS(t *testing.T) {
	// Mock osStatFunc
	origOsStat := osStatFunc
	defer func() { osStatFunc = origOsStat }()

	baseDir := "/tmp/base"
	fs := &BasePathFS{Base: baseDir}

	t.Run("File Exists", func(t *testing.T) {
		osStatFunc = func(name string) (os.FileInfo, error) {
			if name == filepath.Join(baseDir, "test.txt") {
				return &mockFileInfo{name: "test.txt"}, nil
			}
			return nil, os.ErrNotExist
		}

		info, err := fs.Stat("test.txt")
		require.NoError(t, err)
		assert.Equal(t, "test.txt", info.Name())
	})

	t.Run("File Not Found", func(t *testing.T) {
		osStatFunc = func(name string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		}

		_, err := fs.Stat("missing.txt")
		assert.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestRunArchitectCmd(t *testing.T) {
	// Backup mocks
	origReadFile := readFileFunc
	origWriteFile := writeFileFunc
	origMkdirAll := mkdirAllFunc
	origOsStat := osStatFunc
	origGetAgent := getAgentClientFunc // Use the package level variable

	defer func() {
		readFileFunc = origReadFile
		writeFileFunc = origWriteFile
		mkdirAllFunc = origMkdirAll
		osStatFunc = origOsStat
		getAgentClientFunc = origGetAgent
	}()

	// Setup virtual filesystem
	files := make(map[string][]byte)

	readFileFunc = func(name string) ([]byte, error) {
		if content, ok := files[name]; ok {
			return content, nil
		}
		return nil, os.ErrNotExist
	}

	writeFileFunc = func(name string, data []byte, perm os.FileMode) error {
		files[name] = data
		return nil
	}

	mkdirAllFunc = func(path string, perm os.FileMode) error {
		return nil
	}

	osStatFunc = func(name string) (os.FileInfo, error) {
		if _, ok := files[name]; ok {
			return &mockFileInfo{name: filepath.Base(name)}, nil
		}
		return nil, os.ErrNotExist
	}

	// Setup Spec
	specFile := "app_spec.txt"
	files[specFile] = []byte("App Spec Content")

	outDir := ".recac/architecture"

	t.Run("Success", func(t *testing.T) {
		// Mock Agent with VALID architecture
		// Requires ID, Name, Type at minimum for component
		validArch := `
system_name: "Test System"
version: "1.0"
components:
  - id: "api-service"
    name: "api"
    type: "service"
    description: "API Service"
`
		mockAgent := &MockArchitectAgent{
			Response: `
` + "```json" + `
{
	"architecture.yaml": ` + fmt.Sprintf("%q", validArch) + `
}
` + "```",
		}

		getAgentClientFunc = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return mockAgent, nil
		}

		// Run Command
		architectCmd.Flags().Set("spec", specFile)
		architectCmd.Flags().Set("out", outDir)

		err := runArchitectCmd(architectCmd, []string{})
		assert.NoError(t, err)

		// Verify output
		assert.Contains(t, files, filepath.Join(outDir, "architecture.yaml"))
	})

	t.Run("Spec Missing", func(t *testing.T) {
		architectCmd.Flags().Set("spec", "missing.txt")
		err := runArchitectCmd(architectCmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error reading spec")
	})

	t.Run("Agent Init Error", func(t *testing.T) {
		architectCmd.Flags().Set("spec", specFile)
		getAgentClientFunc = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return nil, fmt.Errorf("init error")
		}

		err := runArchitectCmd(architectCmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error initializing agent")
	})

	t.Run("Validation Failure", func(t *testing.T) {
		// Agent returns invalid architecture (invalid yaml)
		mockAgent := &MockArchitectAgent{
			Response: `
` + "```json" + `
{
	"architecture.yaml": ": invalid yaml"
}
` + "```",
		}
		getAgentClientFunc = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return mockAgent, nil
		}

		architectCmd.Flags().Set("spec", specFile)
		err := runArchitectCmd(architectCmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse architecture.yaml")
	})
}
