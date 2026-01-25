package scenarios

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenericScenario_Basics(t *testing.T) {
	config := GenericScenarioConfig{
		Name:        "Test Scenario",
		Description: "A test scenario",
		AppSpec:     "Spec for {{.RepoURL}}",
	}
	s := NewGenericScenario(config)

	assert.Equal(t, "Test Scenario", s.Name())
	assert.Equal(t, "A test scenario", s.Description())
	assert.Equal(t, "Spec for http://repo.url", s.AppSpec("http://repo.url"))
}

func TestGenericScenario_Generate(t *testing.T) {
	config := GenericScenarioConfig{
		Tickets: []TicketTemplate{
			{
				ID:      "TICKET-1",
				Summary: "Fix {{.UniqueID}}",
				Desc:    "Description for {{.RepoURL}}",
				Type:    "Bug",
			},
		},
	}
	s := NewGenericScenario(config)

	specs := s.Generate("123", "http://repo")
	assert.Len(t, specs, 1)
	assert.Equal(t, "Fix 123", specs[0].Summary)
	assert.Equal(t, "Description for http://repo", specs[0].Desc)
	assert.Equal(t, "TICKET-1", specs[0].ID)
}

func TestGenericScenario_RunStep_FileExists(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(filePath, []byte("content"), 0644)

	s := NewGenericScenario(GenericScenarioConfig{})

	// Pass
	step := ValidationStep{
		Type: ValidateFileExists,
		Path: "test.txt",
	}
	err := s.runStep(tmpDir, step)
	assert.NoError(t, err)

	// Fail
	stepFail := ValidationStep{
		Type: ValidateFileExists,
		Path: "missing.txt",
	}
	err = s.runStep(tmpDir, stepFail)
	assert.Error(t, err)
}

func TestGenericScenario_RunStep_FileContent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(filePath, []byte("hello world"), 0644)

	s := NewGenericScenario(GenericScenarioConfig{})

	// Pass Match
	err := s.runStep(tmpDir, ValidationStep{
		Type:             ValidateFileContent,
		Path:             "test.txt",
		ContentMustMatch: "hello",
	})
	assert.NoError(t, err)

	// Pass Not Match
	err = s.runStep(tmpDir, ValidationStep{
		Type:                ValidateFileContent,
		Path:                "test.txt",
		ContentMustNotMatch: "foo",
	})
	assert.NoError(t, err)

	// Fail Match
	err = s.runStep(tmpDir, ValidationStep{
		Type:             ValidateFileContent,
		Path:             "test.txt",
		ContentMustMatch: "foo",
	})
	assert.Error(t, err)

	// Fail Not Match
	err = s.runStep(tmpDir, ValidationStep{
		Type:                ValidateFileContent,
		Path:                "test.txt",
		ContentMustNotMatch: "world",
	})
	assert.Error(t, err)
}

func TestGenericScenario_RunStep_RunCommand(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewGenericScenario(GenericScenarioConfig{})

	// Pass: echo hello
	err := s.runStep(tmpDir, ValidationStep{
		Type:             ValidateRunCommand,
		Path:             "echo",
		Args:             []string{"hello"},
		ContentMustMatch: "hello",
	})
	assert.NoError(t, err)

	// Fail: exit code (ls missing file)
	err = s.runStep(tmpDir, ValidationStep{
		Type: ValidateRunCommand,
		Path: "ls",
		Args: []string{"nonexistentfile"},
	})
	assert.Error(t, err)
}

func TestGenericScenario_Verify_ValidationFail(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewGenericScenario(GenericScenarioConfig{
		Validations: []ValidationStep{
			{
				Name: "FailStep",
				Type: ValidateFileExists,
				Path: "missing.txt",
			},
		},
	})

	err := s.Verify(tmpDir, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation 'FailStep' failed")
}
