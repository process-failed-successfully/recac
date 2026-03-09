package scenarios

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGenericScenario_Verify(t *testing.T) {
	// Let's create a git repository locally to test the verify checkout flow
	dir := setupGitRepo(t)

	// We need remote origin
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

	// Create a dummy ticket ID branch
	_ = exec.Command("git", "-C", dir, "branch", "agent/T-123").Run()
	_ = exec.Command("git", "-C", dir, "push", "origin", "agent/T-123").Run()
	_ = exec.Command("git", "-C", dir, "checkout", "agent/T-123").Run()

	// Create dummy file for validation
	_ = os.WriteFile(filepath.Join(dir, "exists.txt"), []byte("Hello World"), 0644)

	config := GenericScenarioConfig{
		Name:        "Test Verify Scenario",
		Description: "A scenario to test verify",
		AppSpec:     "Test AppSpec",
		Tickets: []TicketTemplate{
			{ID: "T-123"},
		},
		Validations: []ValidationStep{
			{
				Name: "Check file exists",
				Type: ValidateFileExists,
				Path: "exists.txt",
			},
		},
	}
	s := NewGenericScenario(config)

	// Try without tickets mapping
	err := s.Verify(dir, nil)
	if err != nil {
		t.Fatalf("Verify without ticket map failed: %v", err)
	}

	if s.Description() != "A scenario to test verify" {
		t.Fatalf("Description mismatch")
	}

	// Try with map matching the branch
	err = s.Verify(dir, map[string]string{"T-123": "T-123"})
	if err != nil {
		t.Fatalf("Verify with ticket map failed: %v", err)
	}

	// Create another scenario that has an optional failing validation
	config2 := GenericScenarioConfig{
		Name: "Test Verify Scenario Fail Optional",
		Validations: []ValidationStep{
			{
				Name:     "Check file missing",
				Type:     ValidateFileExists,
				Path:     "missing.txt",
				Optional: true,
			},
		},
	}
	s2 := NewGenericScenario(config2)
	err = s2.Verify(dir, nil)
	if err != nil {
		t.Fatalf("Verify failed but step was optional: %v", err)
	}

	// Create another scenario that has a failing validation
	config3 := GenericScenarioConfig{
		Name: "Test Verify Scenario Fail",
		Validations: []ValidationStep{
			{
				Name:     "Check file missing",
				Type:     ValidateFileExists,
				Path:     "missing.txt",
				Optional: false,
			},
		},
	}
	s3 := NewGenericScenario(config3)
	err = s3.Verify(dir, nil)
	if err == nil {
		t.Fatalf("Verify passed but step was required and failed")
	}
}

func TestGenericScenario_runStep_Unknown(t *testing.T) {
	s := NewGenericScenario(GenericScenarioConfig{})
	step := ValidationStep{
		Type: "UnknownType",
	}
	err := s.runStep(".", step)
	if err == nil {
		t.Fatalf("Expected error for unknown type")
	}
}
