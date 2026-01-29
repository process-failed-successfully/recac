package scenarios

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericScenario_AppSpec(t *testing.T) {
	config := GenericScenarioConfig{
		AppSpec: "Spec for {{.RepoURL}}",
	}
	s := NewGenericScenario(config)
	got := s.AppSpec("http://repo.git")
	if got != "Spec for http://repo.git" {
		t.Errorf("AppSpec mismatch, got %q", got)
	}
}

func TestGenericScenario_Generate(t *testing.T) {
	config := GenericScenarioConfig{
		Tickets: []TicketTemplate{
			{
				ID:      "T1",
				Summary: "Sum {{.UniqueID}}",
				Desc:    "Desc {{.RepoURL}}",
				Type:    "Task",
			},
		},
	}
	s := NewGenericScenario(config)
	specs := s.Generate("123", "http://repo.git")

	if len(specs) != 1 {
		t.Fatalf("Expected 1 spec, got %d", len(specs))
	}
	if specs[0].Summary != "Sum 123" {
		t.Errorf("Summary mismatch: %q", specs[0].Summary)
	}
	if specs[0].Desc != "Desc http://repo.git" {
		t.Errorf("Desc mismatch: %q", specs[0].Desc)
	}
}

func TestGenericScenario_Verify_Validation(t *testing.T) {
	// Setup temp repo
	tmpDir := t.TempDir()

	// Create some files
	os.WriteFile(filepath.Join(tmpDir, "exist.txt"), []byte("hello world"), 0644)

	config := GenericScenarioConfig{
		// No tickets to skip git checkout logic
		Tickets: []TicketTemplate{},
		Validations: []ValidationStep{
			{
				Name: "Check Exists",
				Type: ValidateFileExists,
				Path: "exist.txt",
			},
			{
				Name: "Check Content",
				Type: ValidateFileContent,
				Path: "exist.txt",
				ContentMustMatch: "world",
				ContentMustNotMatch: "bad",
			},
			{
				Name: "Check Command",
				Type: ValidateRunCommand,
				Path: "echo",
				Args: []string{"foo"},
				ContentMustMatch: "foo",
			},
		},
	}

	s := NewGenericScenario(config)
	err := s.Verify(tmpDir, nil)
	if err != nil {
		t.Errorf("Verify failed: %v", err)
	}
}

func TestGenericScenario_Verify_Failure(t *testing.T) {
	tmpDir := t.TempDir()

	config := GenericScenarioConfig{
		Tickets: []TicketTemplate{},
		Validations: []ValidationStep{
			{
				Name: "Check Missing",
				Type: ValidateFileExists,
				Path: "missing.txt",
			},
		},
	}

	s := NewGenericScenario(config)
	err := s.Verify(tmpDir, nil)
	if err == nil {
		t.Error("Expected error for missing file")
	}
}

func TestGenericScenario_Verify_ContentFailure(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("hello"), 0644)

	config := GenericScenarioConfig{
		Tickets: []TicketTemplate{},
		Validations: []ValidationStep{
			{
				Name: "Check Content",
				Type: ValidateFileContent,
				Path: "file.txt",
				ContentMustMatch: "world",
			},
		},
	}

	s := NewGenericScenario(config)
	err := s.Verify(tmpDir, nil)
	if err == nil {
		t.Error("Expected error for content mismatch")
	}
	if !strings.Contains(err.Error(), "does not contain 'world'") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestGenericScenario_Verify_ForbiddenContent(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("hello world"), 0644)

	config := GenericScenarioConfig{
		Tickets: []TicketTemplate{},
		Validations: []ValidationStep{
			{
				Name: "Check Forbidden",
				Type: ValidateFileContent,
				Path: "file.txt",
				ContentMustNotMatch: "hello",
			},
		},
	}

	s := NewGenericScenario(config)
	err := s.Verify(tmpDir, nil)
	if err == nil {
		t.Error("Expected error for forbidden content")
	}
}
