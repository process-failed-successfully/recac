package scenarios

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericScenario_Generate(t *testing.T) {
	config := GenericScenarioConfig{
		Name: "Test Scenario",
		Tickets: []TicketTemplate{
			{
				ID:      "TEST-1",
				Summary: "Summary for {{.UniqueID}}",
				Desc:    "Repo: {{.RepoURL}}",
				Type:    "Task",
			},
		},
	}
	s := NewGenericScenario(config)

	specs := s.Generate("12345", "https://example.com/repo")

	if len(specs) != 1 {
		t.Fatalf("Expected 1 ticket spec, got %d", len(specs))
	}
	if specs[0].Summary != "Summary for 12345" {
		t.Errorf("Expected summary 'Summary for 12345', got '%s'", specs[0].Summary)
	}
	if specs[0].Desc != "Repo: https://example.com/repo" {
		t.Errorf("Expected desc 'Repo: https://example.com/repo', got '%s'", specs[0].Desc)
	}
}

func TestGenericScenario_AppSpec(t *testing.T) {
	config := GenericScenarioConfig{
		Name:    "Test Scenario",
		AppSpec: "Spec for {{.RepoURL}}",
	}
	s := NewGenericScenario(config)

	spec := s.AppSpec("https://example.com/repo")

	if spec != "Spec for https://example.com/repo" {
		t.Errorf("Expected 'Spec for https://example.com/repo', got '%s'", spec)
	}
}

func TestGenericScenario_RunStep_FileContent(t *testing.T) {
	// Setup generic scenario with no tickets needed for this specific test
	s := NewGenericScenario(GenericScenarioConfig{})

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(filePath, []byte("Hello World"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Case 1: Success match
	step := ValidationStep{
		Name:             "Check Content",
		Type:             ValidateFileContent,
		Path:             "test.txt",
		ContentMustMatch: "Hello",
	}
	if err := s.runStep(tmpDir, step); err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	// Case 2: Fail match
	stepFail := ValidationStep{
		Name:             "Check Missing",
		Type:             ValidateFileContent,
		Path:             "test.txt",
		ContentMustMatch: "Missing",
	}
	if err := s.runStep(tmpDir, stepFail); err == nil {
		t.Error("Expected error for missing content, got nil")
	} else if !strings.Contains(err.Error(), "does not contain 'Missing'") {
		t.Errorf("Expected specific error message, got: %v", err)
	}

	// Case 3: Fail forbidden
	stepForbidden := ValidationStep{
		Name:                "Check Forbidden",
		Type:                ValidateFileContent,
		Path:                "test.txt",
		ContentMustNotMatch: "World",
	}
	if err := s.runStep(tmpDir, stepForbidden); err == nil {
		t.Error("Expected error for forbidden content, got nil")
	}
}

func TestGenericScenario_RunStep_FileExists(t *testing.T) {
	s := NewGenericScenario(GenericScenarioConfig{})
	tmpDir := t.TempDir()

	// Create a file
	existingFile := filepath.Join(tmpDir, "exists.txt")
	if err := os.WriteFile(existingFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test Exist Success
	stepExist := ValidationStep{
		Name: "Check Exists",
		Type: ValidateFileExists,
		Path: "exists.txt",
	}
	if err := s.runStep(tmpDir, stepExist); err != nil {
		t.Errorf("Expected success for existing file, got %v", err)
	}

	// Test Exist Fail
	stepMissing := ValidationStep{
		Name: "Check Missing",
		Type: ValidateFileExists,
		Path: "missing.txt",
	}
	if err := s.runStep(tmpDir, stepMissing); err == nil {
		t.Error("Expected error for missing file, got nil")
	}
}

func TestGenericScenario_RunStep_RunCommand(t *testing.T) {
	// Skip if running in environment where basic unix tools might be missing,
	// but assuming docker/linux environment for now.
	s := NewGenericScenario(GenericScenarioConfig{})
	tmpDir := t.TempDir()

	// Test Echo
	stepEcho := ValidationStep{
		Name:             "Echo Test",
		Type:             ValidateRunCommand,
		Path:             "echo",
		Args:             []string{"hello"},
		ContentMustMatch: "hello",
	}
	if err := s.runStep(tmpDir, stepEcho); err != nil {
		t.Errorf("Expected success for echo command, got %v", err)
	}

	// Test Echo Fail
	stepEchoFail := ValidationStep{
		Name:             "Echo Fail",
		Type:             ValidateRunCommand,
		Path:             "echo",
		Args:             []string{"bye"},
		ContentMustMatch: "hello",
	}
	if err := s.runStep(tmpDir, stepEchoFail); err == nil {
		t.Error("Expected error for mismatched output, got nil")
	}
}
