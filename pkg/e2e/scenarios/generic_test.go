package scenarios

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenericScenario_AppSpec(t *testing.T) {
	config := GenericScenarioConfig{
		AppSpec: "Repo is {{.RepoURL}}",
	}
	s := NewGenericScenario(config)

	spec := s.AppSpec("http://example.com")
	assert.Equal(t, "Repo is http://example.com", spec)

	// Test Invalid Template
	s.Config.AppSpec = "{{.Invalid"
	spec = s.AppSpec("http://example.com")
	assert.Contains(t, spec, "ERROR PARSING APPSPEC TEMPLATE")
}

func TestGenericScenario_Generate(t *testing.T) {
	config := GenericScenarioConfig{
		Tickets: []TicketTemplate{
			{
				ID:      "T1",
				Summary: "Fix {{.UniqueID}}",
				Desc:    "Desc {{.RepoURL}}",
				Type:    "Bug",
			},
		},
	}
	s := NewGenericScenario(config)

	tickets := s.Generate("123", "http://repo")
	assert.Len(t, tickets, 1)
	assert.Equal(t, "Fix 123", tickets[0].Summary)
	assert.Equal(t, "Desc http://repo", tickets[0].Desc)

	// Test Invalid Template
	s.Config.Tickets[0].Summary = "{{.Invalid"
	tickets = s.Generate("123", "http://repo")
	assert.Contains(t, tickets[0].Summary, "ERROR PARSING TEMPLATE")
}

func TestGenericScenario_RunStep(t *testing.T) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "generic_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	s := NewGenericScenario(GenericScenarioConfig{})

	t.Run("ValidateFileExists", func(t *testing.T) {
		// Create file
		fPath := filepath.Join(tmpDir, "exist.txt")
		os.WriteFile(fPath, []byte("content"), 0644)

		step := ValidationStep{
			Type: ValidateFileExists,
			Path: "exist.txt",
		}
		err := s.runStep(tmpDir, step)
		assert.NoError(t, err)

		step.Path = "nonexistent.txt"
		err = s.runStep(tmpDir, step)
		assert.Error(t, err)
	})

	t.Run("ValidateFileContent", func(t *testing.T) {
		fPath := filepath.Join(tmpDir, "content.txt")
		os.WriteFile(fPath, []byte("hello world"), 0644)

		// Positive Match
		step := ValidationStep{
			Type:             ValidateFileContent,
			Path:             "content.txt",
			ContentMustMatch: "hello",
		}
		err := s.runStep(tmpDir, step)
		assert.NoError(t, err)

		// Negative Match (Should Fail)
		step.ContentMustMatch = "goodbye"
		err = s.runStep(tmpDir, step)
		assert.Error(t, err)

		// Forbidden Content (Should Fail)
		step.ContentMustMatch = ""
		step.ContentMustNotMatch = "world"
		err = s.runStep(tmpDir, step)
		assert.Error(t, err)

		// Forbidden Content (Should Pass)
		step.ContentMustNotMatch = "universe"
		err = s.runStep(tmpDir, step)
		assert.NoError(t, err)
	})

	t.Run("ValidateRunCommand", func(t *testing.T) {
		// Valid command
		step := ValidationStep{
			Type: ValidateRunCommand,
			Path: "echo",
			Args: []string{"hello"},
			ContentMustMatch: "hello",
		}
		err := s.runStep(tmpDir, step)
		assert.NoError(t, err)

		// Invalid command
		step.Path = "nonexistentcommand"
		err = s.runStep(tmpDir, step)
		assert.Error(t, err)

		// Output mismatch
		step.Path = "echo"
		step.Args = []string{"hello"}
		step.ContentMustMatch = "goodbye"
		err = s.runStep(tmpDir, step)
		assert.Error(t, err)
	})

	t.Run("Unknown Validation Type", func(t *testing.T) {
		step := ValidationStep{
			Type: "Unknown",
		}
		err := s.runStep(tmpDir, step)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown validation type")
	})
}

func TestGenericScenario_BasicMethods(t *testing.T) {
	config := GenericScenarioConfig{
		Name:        "TestName",
		Description: "TestDesc",
	}
	s := NewGenericScenario(config)
	assert.Equal(t, "TestName", s.Name())
	assert.Equal(t, "TestDesc", s.Description())
}

func TestGenericScenario_Verify_Optional(t *testing.T) {
	// Verify shouldn't fail if optional step fails
	tmpDir, err := os.MkdirTemp("", "generic_test_optional")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := GenericScenarioConfig{
		Validations: []ValidationStep{
			{
				Name:     "Optional Fail",
				Type:     ValidateFileExists,
				Path:     "missing.txt",
				Optional: true,
			},
		},
	}
	s := NewGenericScenario(config)

	err = s.Verify(tmpDir, nil)
	assert.NoError(t, err)
}

func TestGenericScenario_Verify_Fail(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "generic_test_fail")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := GenericScenarioConfig{
		Validations: []ValidationStep{
			{
				Name: "Required Fail",
				Type: ValidateFileExists,
				Path: "missing.txt",
			},
		},
	}
	s := NewGenericScenario(config)

	err = s.Verify(tmpDir, nil)
	assert.Error(t, err)
}
