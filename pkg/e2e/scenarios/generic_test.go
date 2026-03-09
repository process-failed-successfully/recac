package scenarios

import (
	"os"
	"os/exec"
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

func TestGenericScenario_Verify_Failure(t *testing.T) {
	config := GenericScenarioConfig{
		Name: "Test Verify Fail",
		Tickets: []TicketTemplate{
			{ID: "T-1"},
		},
		Validations: []ValidationStep{
			{
				Name: "Check Missing File",
				Type: ValidateFileExists,
				Path: "missing_file.txt",
			},
		},
	}
	s := NewGenericScenario(config)

	dir := setupGitRepo(t)
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

	exec.Command("git", "-C", dir, "branch", "agent/T-1-fix").Run()
	exec.Command("git", "-C", dir, "push", "origin", "agent/T-1-fix").Run()

	err := s.Verify(dir, map[string]string{"T-1": "T-1"})
	if err == nil {
		t.Errorf("Expected verification to fail")
	}
}

func TestGenericScenario_Verify_Success(t *testing.T) {
	config := GenericScenarioConfig{
		Name: "Test Verify Success",
		Tickets: []TicketTemplate{
			{ID: "T-1"},
		},
		Validations: []ValidationStep{
			{
				Name: "Check Existing File",
				Type: ValidateFileExists,
				Path: "existing_file.txt",
			},
		},
	}
	s := NewGenericScenario(config)

	dir := setupGitRepo(t)
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

	exec.Command("git", "-C", dir, "branch", "agent/T-1-fix").Run()
	exec.Command("git", "-C", dir, "push", "origin", "agent/T-1-fix").Run()

	os.WriteFile(filepath.Join(dir, "existing_file.txt"), []byte("data"), 0644)

	err := s.Verify(dir, map[string]string{"T-1": "T-1"})
	if err != nil {
		t.Errorf("Expected verification to succeed, got %v", err)
	}
}

func TestGenericScenario_Verify_OptionalFailure(t *testing.T) {
	config := GenericScenarioConfig{
		Name: "Test Verify Optional",
		Tickets: []TicketTemplate{
			{ID: "T-1"},
		},
		Validations: []ValidationStep{
			{
				Name:     "Check Missing File Optional",
				Type:     ValidateFileExists,
				Path:     "missing_file.txt",
				Optional: true,
			},
		},
	}
	s := NewGenericScenario(config)

	dir := setupGitRepo(t)
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

	exec.Command("git", "-C", dir, "branch", "agent/T-1-fix").Run()
	exec.Command("git", "-C", dir, "push", "origin", "agent/T-1-fix").Run()

	err := s.Verify(dir, map[string]string{"T-1": "T-1"})
	if err != nil {
		t.Errorf("Expected verification to succeed despite optional failure, got %v", err)
	}
}

func TestGenericScenario_RunStep_InvalidType(t *testing.T) {
	s := NewGenericScenario(GenericScenarioConfig{})
	tmpDir := t.TempDir()

	step := ValidationStep{
		Type: "UnknownType",
	}
	if err := s.runStep(tmpDir, step); err == nil {
		t.Error("Expected error for unknown validation type")
	}
}

func TestGenericScenario_AppSpec_TemplateError(t *testing.T) {
	config := GenericScenarioConfig{
		AppSpec: "Spec for repo {{.InvalidSyntax",
	}
	s := NewGenericScenario(config)

	spec := s.AppSpec("http://repo.com")
	if !strings.Contains(spec, "ERROR PARSING APPSPEC TEMPLATE") {
		t.Errorf("Expected error string, got: %s", spec)
	}
}

func TestGenericScenario_Generate_TemplateError(t *testing.T) {
	config := GenericScenarioConfig{
		Tickets: []TicketTemplate{
			{
				ID:      "T1",
				Summary: "Sum {{.InvalidSyntax",
				Desc:    "Desc {{.InvalidSyntax",
			},
		},
	}
	s := NewGenericScenario(config)

	specs := s.Generate("123", "http://repo")
	if len(specs) != 1 {
		t.Fatalf("expected 1 ticket, got %d", len(specs))
	}

	if !strings.Contains(specs[0].Summary, "ERROR PARSING TEMPLATE") {
		t.Errorf("Expected parse error for summary, got: %s", specs[0].Summary)
	}
	if !strings.Contains(specs[0].Desc, "ERROR PARSING TEMPLATE") {
		t.Errorf("Expected parse error for desc, got: %s", specs[0].Desc)
	}
}
