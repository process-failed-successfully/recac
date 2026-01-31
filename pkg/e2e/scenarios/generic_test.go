package scenarios

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenericScenario_Basic(t *testing.T) {
	cfg := GenericScenarioConfig{
		Name:        "Test Scenario",
		Description: "A test description",
		AppSpec:     "App Spec for {{.RepoURL}}",
	}
	s := NewGenericScenario(cfg)

	if s.Name() != "Test Scenario" {
		t.Errorf("Expected name 'Test Scenario', got '%s'", s.Name())
	}
	if s.Description() != "A test description" {
		t.Errorf("Expected description 'A test description', got '%s'", s.Description())
	}

	spec := s.AppSpec("http://repo.url")
	if spec != "App Spec for http://repo.url" {
		t.Errorf("AppSpec template expansion failed: %s", spec)
	}
}

func TestGenericScenario_Generate(t *testing.T) {
	cfg := GenericScenarioConfig{
		Tickets: []TicketTemplate{
			{
				ID:      "T-1",
				Summary: "Summary {{.UniqueID}}",
				Desc:    "Desc {{.RepoURL}}",
				Type:    "Task",
			},
		},
	}
	s := NewGenericScenario(cfg)

	tickets := s.Generate("uid123", "http://repo")
	if len(tickets) != 1 {
		t.Fatalf("Expected 1 ticket, got %d", len(tickets))
	}

	if tickets[0].Summary != "Summary uid123" {
		t.Errorf("Summary expansion failed: %s", tickets[0].Summary)
	}
	if tickets[0].Desc != "Desc http://repo" {
		t.Errorf("Desc expansion failed: %s", tickets[0].Desc)
	}
}

func TestGenericScenario_Verify_FileExists(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "scenario_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a file
	filename := "testfile.txt"
	if err := os.WriteFile(filepath.Join(tmpDir, filename), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := GenericScenarioConfig{
		Validations: []ValidationStep{
			{
				Name: "Check File",
				Type: ValidateFileExists,
				Path: filename,
			},
		},
	}
	s := NewGenericScenario(cfg)

	if err := s.Verify(tmpDir, nil); err != nil {
		t.Errorf("Verify failed for existing file: %v", err)
	}

	// Negative test
	cfg.Validations[0].Path = "missing.txt"
	if err := s.Verify(tmpDir, nil); err == nil {
		t.Error("Verify should fail for missing file")
	}
}

func TestGenericScenario_Verify_FileContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scenario_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	filename := "testfile.txt"
	content := "foo bar baz"
	if err := os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Success case
	cfg := GenericScenarioConfig{
		Validations: []ValidationStep{
			{
				Name:             "Check Content",
				Type:             ValidateFileContent,
				Path:             filename,
				ContentMustMatch: "bar",
			},
		},
	}
	s := NewGenericScenario(cfg)
	if err := s.Verify(tmpDir, nil); err != nil {
		t.Errorf("Verify failed for matching content: %v", err)
	}

	// Fail case (MustMatch)
	cfg.Validations[0].ContentMustMatch = "qux"
	if err := s.Verify(tmpDir, nil); err == nil {
		t.Error("Verify should fail when content missing")
	}

	// Fail case (MustNotMatch)
	cfg.Validations[0].ContentMustMatch = ""
	cfg.Validations[0].ContentMustNotMatch = "foo"
	if err := s.Verify(tmpDir, nil); err == nil {
		t.Error("Verify should fail when forbidden content present")
	}
}

func TestGenericScenario_Verify_RunCommand(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scenario_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// We'll use 'echo' command which should exist
	cfg := GenericScenarioConfig{
		Validations: []ValidationStep{
			{
				Name:             "Check Echo",
				Type:             ValidateRunCommand,
				Path:             "echo",
				Args:             []string{"hello world"},
				ContentMustMatch: "hello",
			},
		},
	}
	s := NewGenericScenario(cfg)

	if err := s.Verify(tmpDir, nil); err != nil {
		t.Errorf("Verify failed for command: %v", err)
	}

    // Fail case
    cfg.Validations[0].ContentMustMatch = "goodbye"
    if err := s.Verify(tmpDir, nil); err == nil {
        t.Error("Verify should fail for matching content mismatch")
    }
}
