package scenarios

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenericScenario_Basic(t *testing.T) {
	cfg := GenericScenarioConfig{
		Name:        "Test Scenario",
		Description: "A test description",
		AppSpec:     "Spec for {{.RepoURL}}",
	}
	s := NewGenericScenario(cfg)

	assert.Equal(t, "Test Scenario", s.Name())
	assert.Equal(t, "A test description", s.Description())
	assert.Equal(t, "Spec for https://repo.com", s.AppSpec("https://repo.com"))
}

func TestGenericScenario_Verify_FileExists(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "exist.txt"), []byte("content"), 0644)

	cfg := GenericScenarioConfig{
		Validations: []ValidationStep{
			{
				Name: "Check Exist",
				Type: ValidateFileExists,
				Path: "exist.txt",
			},
			{
				Name: "Check Missing",
				Type: ValidateFileExists,
				Path: "missing.txt",
				Optional: true, // Should fail but not error? Verify returns error if not optional.
				// Verify prints warning if Optional=true.
			},
		},
	}
	s := NewGenericScenario(cfg)

	err := s.Verify(tmpDir, nil)
	assert.NoError(t, err) // Optional failure shouldn't cause error

	// Test failure
	cfg.Validations[1].Optional = false
	s = NewGenericScenario(cfg)
	err = s.Verify(tmpDir, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file not found: missing.txt")
}

func TestGenericScenario_Verify_FileContent(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("Hello World"), 0644)

	cfg := GenericScenarioConfig{
		Validations: []ValidationStep{
			{
				Name:             "Check Content",
				Type:             ValidateFileContent,
				Path:             "file.txt",
				ContentMustMatch: "World",
			},
			{
				Name:                "Check Forbidden",
				Type:                ValidateFileContent,
				Path:                "file.txt",
				ContentMustNotMatch: "Foo",
			},
		},
	}
	s := NewGenericScenario(cfg)

	err := s.Verify(tmpDir, nil)
	assert.NoError(t, err)

	// Failure cases
	cfg.Validations = []ValidationStep{
		{
			Name:             "Fail Match",
			Type:             ValidateFileContent,
			Path:             "file.txt",
			ContentMustMatch: "Universe",
		},
	}
	s = NewGenericScenario(cfg)
	err = s.Verify(tmpDir, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not contain 'Universe'")
}
