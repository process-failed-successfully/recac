package scenarios

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGenericScenario(t *testing.T) {
	config := GenericScenarioConfig{
		Name:        "Test Scenario",
		Description: "A test scenario",
	}
	s := NewGenericScenario(config)
	assert.Equal(t, "Test Scenario", s.Name())
	assert.Equal(t, "A test scenario", s.Description())
}

func TestGenericScenario_AppSpec(t *testing.T) {
	config := GenericScenarioConfig{
		AppSpec: "Repo: {{.RepoURL}}",
	}
	s := NewGenericScenario(config)
	spec := s.AppSpec("http://example.com/repo")
	assert.Equal(t, "Repo: http://example.com/repo", spec)
}

func TestGenericScenario_Generate(t *testing.T) {
	config := GenericScenarioConfig{
		Tickets: []TicketTemplate{
			{
				ID:      "T1",
				Type:    "Task",
				Summary: "Fix {{.UniqueID}}",
				Desc:    "Description for {{.RepoURL}}",
			},
		},
	}
	s := NewGenericScenario(config)
	tickets := s.Generate("123", "http://repo")

	assert.Len(t, tickets, 1)
	assert.Equal(t, "T1", tickets[0].ID)
	assert.Equal(t, "Task", tickets[0].Type)
	assert.Equal(t, "Fix 123", tickets[0].Summary)
	assert.Equal(t, "Description for http://repo", tickets[0].Desc)
}

func TestGenericScenario_RunStep_FileExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-generic-scenario")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a dummy file
	f, err := os.Create(filepath.Join(tmpDir, "test.txt"))
	assert.NoError(t, err)
	f.Close()

	s := NewGenericScenario(GenericScenarioConfig{})

	// Test success
	step := ValidationStep{
		Name: "Check File",
		Type: ValidateFileExists,
		Path: "test.txt",
	}
	err = s.runStep(tmpDir, step)
	assert.NoError(t, err)

	// Test failure
	stepFail := ValidationStep{
		Name: "Check Missing",
		Type: ValidateFileExists,
		Path: "missing.txt",
	}
	err = s.runStep(tmpDir, stepFail)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file not found")
}

func TestGenericScenario_RunStep_FileContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-generic-scenario")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create file with content
	err = os.WriteFile(filepath.Join(tmpDir, "code.go"), []byte("package main\nfunc main() {}"), 0644)
	assert.NoError(t, err)

	s := NewGenericScenario(GenericScenarioConfig{})

	// Success: Match
	err = s.runStep(tmpDir, ValidationStep{
		Type:             ValidateFileContent,
		Path:             "code.go",
		ContentMustMatch: "func main",
	})
	assert.NoError(t, err)

	// Success: Not Match
	err = s.runStep(tmpDir, ValidationStep{
		Type:                ValidateFileContent,
		Path:                "code.go",
		ContentMustNotMatch: "func foo",
	})
	assert.NoError(t, err)

	// Fail: Missing required content
	err = s.runStep(tmpDir, ValidationStep{
		Type:             ValidateFileContent,
		Path:             "code.go",
		ContentMustMatch: "func bar",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not contain 'func bar'")

	// Fail: Contains forbidden content
	err = s.runStep(tmpDir, ValidationStep{
		Type:                ValidateFileContent,
		Path:                "code.go",
		ContentMustNotMatch: "package main",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "contains forbidden text")
}

func TestGenericScenario_RunStep_Command(t *testing.T) {
	tmpDir := os.TempDir() // Commands run in temp dir
	s := NewGenericScenario(GenericScenarioConfig{})

	// Echo command (available on most systems)
	err := s.runStep(tmpDir, ValidationStep{
		Type:             ValidateRunCommand,
		Path:             "echo",
		Args:             []string{"hello world"},
		ContentMustMatch: "hello",
	})
	assert.NoError(t, err)

	// Failure command
	err = s.runStep(tmpDir, ValidationStep{
		Type: ValidateRunCommand,
		Path: "false", // Exits with 1
	})
	assert.Error(t, err)
}
