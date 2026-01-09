package scenarios

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

// ValidationType defines the type of validation to perform.
type ValidationType string

const (
	// ValidateFileExists checks if a file exists.
	ValidateFileExists ValidationType = "FileExists"
	// ValidateFileContent checks if a file's content matches a pattern (contains string).
	ValidateFileContent ValidationType = "FileContent"
	// ValidateRunCommand runs a command and checks for exit code 0 and optional output matching.
	ValidateRunCommand ValidationType = "RunCommand"
	// ValidateGitBranch checks if a specific branch exists/is checked out (often handled implicitly, but explicit check option).
	ValidateGitBranch ValidationType = "GitBranch"
)

// ValidationStep defines a single step in the verification process.
type ValidationStep struct {
	Name                string         // Human-readable name
	Type                ValidationType // Type of validation
	Path                string         // File path or Command to run
	Args                []string       // Arguments for command
	ContentMustMatch    string         // Text that must be present (for FileContent or RunCommand output)
	ContentMustNotMatch string         // Text that must NOT be present
	Optional            bool           // If true, failure doesn't fail the entire test (warns only)
}

// TicketTemplate defines a ticket to be generated using Go templates.
type TicketTemplate struct {
	ID       string   // Internal ID
	Summary  string   // Template string for Summary
	Desc     string   // Template string for Description
	Type     string   // Issue Type
	Blockers []string // List of Internal IDs
}

// GenericScenarioConfig defines the configuration for a generic scenario.
// This struct can be exported or defined in code to create new scenarios.
type GenericScenarioConfig struct {
	Name        string
	Description string
	Tickets     []TicketTemplate
	Validations []ValidationStep
}

// GenericScenario implements the Scenario interface for declarative tests.
type GenericScenario struct {
	Config GenericScenarioConfig
}

// NewGenericScenario creates a new generic scenario from config.
func NewGenericScenario(config GenericScenarioConfig) *GenericScenario {
	return &GenericScenario{Config: config}
}

func (s *GenericScenario) Name() string {
	return s.Config.Name
}

func (s *GenericScenario) Description() string {
	return s.Config.Description
}

// TemplateData holds data passed to templates.
type TemplateData struct {
	UniqueID string
	RepoURL  string
}

func (s *GenericScenario) Generate(uniqueID string, repoURL string) []TicketSpec {
	var specs []TicketSpec
	data := TemplateData{
		UniqueID: uniqueID,
		RepoURL:  repoURL,
	}

	for _, tmpl := range s.Config.Tickets {
		spec := TicketSpec{
			ID:       tmpl.ID,
			Type:     tmpl.Type,
			Blockers: tmpl.Blockers,
		}

		// render summary
		t, err := template.New("summary").Parse(tmpl.Summary)
		if err != nil {
			// In production code, might want to handle error better, but panic/log for test config is acceptable
			spec.Summary = fmt.Sprintf("ERROR PARSING TEMPLATE: %s", tmpl.Summary)
		} else {
			var buf bytes.Buffer
			if err := t.Execute(&buf, data); err != nil {
				spec.Summary = fmt.Sprintf("ERROR EXECUTING TEMPLATE: %v", err)
			} else {
				spec.Summary = buf.String()
			}
		}

		// render description
		tDesc, err := template.New("desc").Parse(tmpl.Desc)
		if err != nil {
			spec.Desc = fmt.Sprintf("ERROR PARSING TEMPLATE: %s", tmpl.Desc)
		} else {
			var buf bytes.Buffer
			if err := tDesc.Execute(&buf, data); err != nil {
				spec.Desc = fmt.Sprintf("ERROR EXECUTING TEMPLATE: %v", err)
			} else {
				spec.Desc = buf.String()
			}
		}

		specs = append(specs, spec)
	}

	return specs
}

func (s *GenericScenario) Verify(repoPath string, ticketKeys map[string]string) error {
	fmt.Printf("Verifying scenario: %s\n", s.Name())

	// Default: Checkout branch for the first ticket if not specified?
	// For now, let's assume we try to checkout the branch for the first Ticket ID defined in config
	if len(s.Config.Tickets) > 0 {
		firstID := s.Config.Tickets[0].ID
		if key, ok := ticketKeys[firstID]; ok {
			// Helper to find specific agent branch (imported from utils if available within same package)
			// Since we are in same package 'scenarios', we can call private funcs if they were exported or public.
			// utils currently has getAgentBranch.

			// We'll trust the agent logic to be on *some* branch.
			// Ideally we use `getSpecificAgentBranch` if available, or just check what branch we are on?
			// Let's reuse logic from prime_python verification roughly: checking out branch.

			// Try to find branch
			branch, err := getSpecificAgentBranch(repoPath, key)
			if err != nil {
				fmt.Printf("Warning: Could not find specific branch for %s (%s), trying generic...\n", firstID, key)
				branch, err = getAgentBranch(repoPath)
				if err != nil {
					return fmt.Errorf("failed to find agent branch: %w", err)
				}
			}

			fmt.Printf("Checking out branch: %s\n", branch)
			checkoutCmd := exec.Command("git", "checkout", branch)
			checkoutCmd.Dir = repoPath
			if out, err := checkoutCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to checkout %s: %v\nOutput: %s", branch, err, out)
			}
		}
	}

	for _, step := range s.Config.Validations {
		fmt.Printf("Running validation: %s (%s)\n", step.Name, step.Type)
		err := s.runStep(repoPath, step)
		if err != nil {
			if step.Optional {
				fmt.Printf("Warning: Optional validation failed: %v\n", err)
			} else {
				return fmt.Errorf("validation '%s' failed: %w", step.Name, err)
			}
		} else {
			fmt.Printf("  -> Passed\n")
		}
	}
	return nil
}

func (s *GenericScenario) runStep(repoPath string, step ValidationStep) error {
	switch step.Type {
	case ValidateFileExists:
		fullPath := filepath.Join(repoPath, step.Path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", step.Path)
		}
		return nil

	case ValidateFileContent:
		fullPath := filepath.Join(repoPath, step.Path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", step.Path, err)
		}
		strContent := string(content)
		if step.ContentMustMatch != "" && !strings.Contains(strContent, step.ContentMustMatch) {
			return fmt.Errorf("file %s does not contain '%s'", step.Path, step.ContentMustMatch)
		}
		if step.ContentMustNotMatch != "" && strings.Contains(strContent, step.ContentMustNotMatch) {
			return fmt.Errorf("file %s contains forbidden text '%s'", step.Path, step.ContentMustNotMatch)
		}
		return nil

	case ValidateRunCommand:
		cmd := exec.Command(step.Path, step.Args...)
		cmd.Dir = repoPath
		outBytes, err := cmd.CombinedOutput()
		output := string(outBytes)

		if err != nil {
			return fmt.Errorf("command execution failed: %v\nOutput: %s", err, output)
		}

		if step.ContentMustMatch != "" && !strings.Contains(output, step.ContentMustMatch) {
			return fmt.Errorf("output missing expected text '%s'. Output was:\n%s", step.ContentMustMatch, output)
		}
		if step.ContentMustNotMatch != "" && strings.Contains(output, step.ContentMustNotMatch) {
			return fmt.Errorf("output contains forbidden text '%s'. Output was:\n%s", step.ContentMustNotMatch, output)
		}

		// If validating JSON output via ContentMustMatch is too brittle, complex structure validation might need a custom step type.
		// Simple string matching is good for now.
		return nil

	default:
		return fmt.Errorf("unknown validation type: %s", step.Type)
	}
}
