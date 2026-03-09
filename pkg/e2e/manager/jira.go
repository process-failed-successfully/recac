package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"recac/internal/jira"
	"recac/pkg/e2e/scenarios"
)

type JiraManager struct {
	Client     *jira.Client
	ProjectKey string
}

func NewJiraManager(baseURL, username, apiToken, projectKey string) *JiraManager {
	client := jira.NewClient(baseURL, username, apiToken)
	return &JiraManager{
		Client:     client,
		ProjectKey: projectKey,
	}
}

func (m *JiraManager) Authenticate(ctx context.Context) error {
	return m.Client.Authenticate(ctx)
}

func (m *JiraManager) GenerateScenario(ctx context.Context, scenarioName, repoURL, provider, model string) (string, map[string]string, error) {
	scenario, ok := scenarios.Registry[scenarioName]
	if !ok {
		return "", nil, fmt.Errorf("unknown scenario: %s", scenarioName)
	}

	uniqueID := time.Now().Format("20060102-150405")
	label := fmt.Sprintf("recac-e2e-%s", uniqueID) // CLI adds recac-gen- prefix to its own label, we add our own.

	// 1. Get AppSpec
	spec := scenario.AppSpec(repoURL)
	specFile := filepath.Join(os.TempDir(), fmt.Sprintf("app_spec_%s.txt", uniqueID))
	outputFile := filepath.Join(os.TempDir(), fmt.Sprintf("tickets_%s.json", uniqueID))

	if err := os.WriteFile(specFile, []byte(spec), 0644); err != nil {
		return "", nil, fmt.Errorf("failed to write spec file: %w", err)
	}
	// defer os.Remove(specFile)
	// defer os.Remove(outputFile)

	// 2. Choose Flow: Legacy (Prompt) or Architect (Validated)
	architectMode := os.Getenv("RECAC_ARCHITECT_MODE") == "true"

	recacCmd := "./recac"
	if _, err := os.Stat("recac"); os.IsNotExist(err) {
		recacCmd = "go run ./cmd/recac"
	}

	if architectMode {
		fmt.Println("=== RUNNING IN ARCHITECT MODE ===")
		archDir := filepath.Join(os.TempDir(), fmt.Sprintf("arch_%s", uniqueID))
		archCmd := fmt.Sprintf("%s architect --spec %s --out %s", recacCmd, specFile, archDir)
		fmt.Printf("Running: %s\n", archCmd)

		// Execute Architect
		var cmd *exec.Cmd
		if strings.HasPrefix(recacCmd, "go run") {
			args := append(strings.Split(recacCmd, " ")[1:], "architect", "--spec", specFile, "--out", archDir, "--provider", provider, "--model", model)
			cmd = exec.Command("go", args...)
		} else {
			cmd = exec.Command(recacCmd, "architect", "--spec", specFile, "--out", archDir, "--provider", provider, "--model", model)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", nil, fmt.Errorf("architect failed: %w", err)
		}

		// Now Generate from Arch
		archYaml := filepath.Join(archDir, "architecture.yaml")
		genCmdArgs := []string{"jira", "generate-from-arch",
			"--arch", archYaml,
			"--spec", specFile,
			"--project", m.ProjectKey,
			"--label", label,
			"--output-json", outputFile,
			"--repo-url", repoURL,
		}

		var genCmd *exec.Cmd
		if strings.HasPrefix(recacCmd, "go run") {
			args := append(strings.Split(recacCmd, " ")[1:], genCmdArgs...)
			genCmd = exec.Command("go", args...)
		} else {
			genCmd = exec.Command(recacCmd, genCmdArgs...)
		}
		genCmd.Stdout = os.Stdout
		genCmd.Stderr = os.Stderr
		if err := genCmd.Run(); err != nil {
			return "", nil, fmt.Errorf("generate-from-arch failed: %w", err)
		}
	} else {
		// Legacy Flow
		cmdArgs := []string{"jira", "generate-from-spec",
			"--spec", specFile,
			"--project", m.ProjectKey,
			"--label", label,
			"--output-json", outputFile,
			"--provider", provider,
			"--model", model,
			"--repo-url", repoURL,
		}

		var cmd *exec.Cmd
		if strings.HasPrefix(recacCmd, "go run") {
			args := append(strings.Split(recacCmd, " ")[1:], cmdArgs...)
			cmd = exec.Command("go", args...)
		} else {
			cmd = exec.Command(recacCmd, cmdArgs...)
		}

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return "", nil, fmt.Errorf("recac cli failed: %w", err)
		}
	}

	// 3. Parse output JSON
	outputData, err := os.ReadFile(outputFile)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read ticket mapping: %w", err)
	}

	var ticketMap map[string]string
	if err := json.Unmarshal(outputData, &ticketMap); err != nil {
		return "", nil, fmt.Errorf("failed to parse ticket mapping: %w", err)
	}

	fmt.Printf("Loaded %d ticket mappings.\n", len(ticketMap))
	for k, v := range ticketMap {
		fmt.Printf("Mapped %s -> %s\n", k, v)
	}

	return label, ticketMap, nil
}


// Cleanup removes all tickets with the given label.
func (m *JiraManager) Cleanup(ctx context.Context, label string) error {
	issues, err := m.Client.LoadLabelIssues(ctx, label)
	if err != nil {
		return fmt.Errorf("failed to load issues for label %s: %w", label, err)
	}

	fmt.Printf("Found %d issues to delete for label %s\n", len(issues), label)
	for _, issue := range issues {
		key, _ := issue["key"].(string)
		if err := m.Client.DeleteIssue(ctx, key); err != nil {
			log.Printf("Failed to delete %s: %v", key, err)
		} else {
			fmt.Printf("Deleted %s\n", key)
		}
	}
	return nil
}
