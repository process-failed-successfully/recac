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

	// Create file
	os.WriteFile(filepath.Join(tmpDir, "exists.txt"), nil, 0644)

	// Success
	step := ValidationStep{
		Type: ValidateFileExists,
		Path: "exists.txt",
	}
	if err := s.runStep(tmpDir, step); err != nil {
		t.Errorf("Expected file to exist: %v", err)
	}

	// Fail
	stepFail := ValidationStep{
		Type: ValidateFileExists,
		Path: "missing.txt",
	}
	if err := s.runStep(tmpDir, stepFail); err == nil {
		t.Error("Expected error for missing file")
	}
}

func TestGenericScenario_RunStep_RunCommand(t *testing.T) {
	if os.Getenv("GOOS") == "windows" {
		t.Skip("Skipping on Windows due to echo/command differences")
	}
	s := NewGenericScenario(GenericScenarioConfig{})
	tmpDir := t.TempDir()

	// Success match
	step := ValidationStep{
		Type:             ValidateRunCommand,
		Path:             "echo",
		Args:             []string{"hello"},
		ContentMustMatch: "hello",
	}
	if err := s.runStep(tmpDir, step); err != nil {
		t.Errorf("Command failed: %v", err)
	}

	// Success forbidden match (should pass if not present)
	step2 := ValidationStep{
		Type:                ValidateRunCommand,
		Path:                "echo",
		Args:                []string{"hello"},
		ContentMustNotMatch: "world",
	}
	if err := s.runStep(tmpDir, step2); err != nil {
		t.Errorf("Command failed: %v", err)
	}

	// Fail match
	stepFail := ValidationStep{
		Type:             ValidateRunCommand,
		Path:             "echo",
		Args:             []string{"hello"},
		ContentMustMatch: "world",
	}
	if err := s.runStep(tmpDir, stepFail); err == nil {
		t.Error("Expected error for mismatched output")
	}

	// Fail command execution
	stepError := ValidationStep{
		Type: ValidateRunCommand,
		Path: "non_existent_command_12345",
	}
	if err := s.runStep(tmpDir, stepError); err == nil {
		t.Error("Expected error for invalid command")
	}
}

func TestGenericScenario_AppSpec(t *testing.T) {
	config := GenericScenarioConfig{
		AppSpec: "Spec for repo {{.RepoURL}}",
	}
	s := NewGenericScenario(config)

	spec := s.AppSpec("http://repo.com")
	if spec != "Spec for repo http://repo.com" {
		t.Errorf("Unexpected spec content: %s", spec)
	}
}

func TestGenericScenario_AppSpec_Errors(t *testing.T) {
	// Parse error
	configParse := GenericScenarioConfig{
		AppSpec: "{{.RepoURL", // Missing closing brace
	}
	sParse := NewGenericScenario(configParse)
	specParse := sParse.AppSpec("http://repo.com")
	if !strings.Contains(specParse, "ERROR PARSING APPSPEC TEMPLATE") {
		t.Errorf("Expected parse error message, got: %s", specParse)
	}

	// Execute error
	configExec := GenericScenarioConfig{
		AppSpec: "{{template \"nonexistent\"}}",
	}
	sExec := NewGenericScenario(configExec)
	specExec := sExec.AppSpec("http://repo.com")
	if !strings.Contains(specExec, "ERROR EXECUTING APPSPEC TEMPLATE") {
		t.Errorf("Expected execute error message, got: %s", specExec)
	}
}

func TestGenericScenario_Generate_Errors(t *testing.T) {
	config := GenericScenarioConfig{
		Name: "Test Scenario",
		Tickets: []TicketTemplate{
			{
				ID:      "TEST-1",
				Summary: "{{.UniqueID",                  // Parse error
				Desc:    "{{template \"nonexistent\"}}", // Execute error
				Type:    "Task",
			},
			{
				ID:      "TEST-2",
				Summary: "{{template \"nonexistent\"}}", // Execute error
				Desc:    "{{.RepoURL",                   // Parse error
				Type:    "Task",
			},
		},
	}
	s := NewGenericScenario(config)

	specs := s.Generate("12345", "https://example.com/repo")

	if len(specs) != 2 {
		t.Fatalf("Expected 2 ticket specs, got %d", len(specs))
	}

	if !strings.Contains(specs[0].Summary, "ERROR PARSING TEMPLATE") {
		t.Errorf("Expected parse error for summary, got: %s", specs[0].Summary)
	}
	if !strings.Contains(specs[0].Desc, "ERROR EXECUTING TEMPLATE") {
		t.Errorf("Expected execute error for desc, got: %s", specs[0].Desc)
	}

	if !strings.Contains(specs[1].Summary, "ERROR EXECUTING TEMPLATE") {
		t.Errorf("Expected execute error for summary, got: %s", specs[1].Summary)
	}
	if !strings.Contains(specs[1].Desc, "ERROR PARSING TEMPLATE") {
		t.Errorf("Expected parse error for desc, got: %s", specs[1].Desc)
	}
}
