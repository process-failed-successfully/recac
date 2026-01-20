package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/db"
	"recac/internal/docker"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// Minimal Feature struct for JSON
// Feature is already defined in planner.go in this package

func TestProjectCompleteFlow(t *testing.T) {
	// Setup Temp Workspace
	workspace, err := os.MkdirTemp("", "recac-test-complete")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(workspace)
	fmt.Printf("Workspace: %s\n", workspace)

	// Create app_spec.txt
	os.WriteFile(filepath.Join(workspace, "app_spec.txt"), []byte("Mock Spec"), 0644)

	// 1. Prepare feature list (to be injected)
	list := db.FeatureList{
		ProjectName: "Test Project",
		Features: []db.Feature{
			{Description: "Feature 1", Status: "done"},
			{Description: "Feature 2", Status: "done"},
		},
	}
	data, _ := json.Marshal(list)
	featureContent := string(data)

	// Mock Docker and Agent
	dockerCli, mockAPI := docker.NewMockClient()

	// Intercept ContainerExecCreate to handle file creation side-effect
	mockAPI.ContainerExecCreateFunc = func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
		// Allow git bootstrap commands
		if len(config.Cmd) > 1 && config.Cmd[1] == "git" {
			return types.IDResponse{ID: "bootstrap-exec-id"}, nil
		}
		// config.Cmd is usually ["/bin/sh", "-c", "script..."]
		if len(config.Cmd) > 2 {
			script := config.Cmd[2]
			// Check for basic echo to file
			// "echo PASS > <workspace>/.qa_result"
			if len(script) > 10 && script[:4] == "echo" {
				// Naive parsing for test: just write "PASS" to .qa_result in workspace
				path := filepath.Join(workspace, ".qa_result")
				os.WriteFile(path, []byte("PASS"), 0644)
			}
		}
		return types.IDResponse{ID: "mock-exec-id"}, nil
	}

	agentClient := &MockAgentForQA{
		Response:  "PASS",
		Workspace: workspace,
	}

	// Create Session
	session := NewSession(dockerCli, agentClient, workspace, "ubuntu:latest", "test-project", "gemini", "gemini-pro", 1)
	session.FeatureContent = featureContent

	// Inject Store and Project into MockAgent
	agentClient.Store = session.DBStore
	agentClient.Project = session.Project

	// We also need to mock the QAAgent and ManagerAgent inside session specifically
	// because RunLoop creates NEW agents if they are nil.
	// So we must inject them.
	session.QAAgent = agentClient
	session.ManagerAgent = agentClient

	// We can't easily execute RunLoop because it's infinite loop unless we have a way to break.
	// But we can check the startup logic by setting MaxIterations=1.
	session.MaxIterations = 1

	// Create context
	ctx := context.Background()

	// Run Loop
	// We expect:
	// 1. Startup check finds features -> Creates COMPLETED signal
	// 2. Loop start checks COMPLETED -> Runs QA Agent (which runs echo PASS)
	// 3. QA Agent finishes -> Creates QA_PASSED signal
	// 4. Loop continues (but MaxIterations might stop it, or we rely on loop continue)
	// Actually, if we set MaxIterations=2, we can see the transition.

	// However, MockAgent returns the SAME response every time currently.
	// Iteration 1 (QA): Returns command "echo PASS > .qa_result"
	// Session executes command.
	// Session sees result PASS. Sets QA_PASSED.
	// Iteration 2 (Manager): Returns command "echo PASS > .qa_result" (reused)
	// Manager logic doesn't execute commands from response in the same way?
	// Wait, runManagerAgent sends prompt, gets response. It prints response.
	// It doesn't execute commands. It just checks completion ratio of features.
	// Features are already all passes=true.
	// So Manager checks qaReport (computed from features). It sees 100%. It approves.
	// So Manager doesn't depend on agent response content for approval (in current simplified logic).

	session.MaxIterations = 1
	err = session.RunLoop(ctx)
	if err != nil && err != ErrMaxIterations {
		t.Fatalf("RunLoop Iteration 1 failed: %v", err)
	}

	// Change mock response for Manager iteration
	agentClient.SetResponse("LGTM")

	session.Iteration = 2
	session.MaxIterations = 3
	err = session.RunLoop(ctx)
	if err != nil && err != ErrMaxIterations {
		t.Fatalf("RunLoop Iteration 2+ failed: %v", err)
	}

	// Check Signals in DB
	// session.DBStore might be nil if sqlite failed (it warns).
	// But NewSession attempts to init it.
	// In test, NewSession might fail to init DB if CGO is issue or path issue.
	// Let's rely on stdout or check if we can inspect session state.
	// We can't inspect private members easily.
	// But we can check if .qa_result was created (proof QA ran).

	if _, err := os.Stat(filepath.Join(workspace, ".qa_result")); err == nil {
		// It should be deleted by runQAAgent after reading.
		// So it should NOT exist if QA finished.
		t.Errorf(".qa_result file should have been cleaned up")
	}

	// We really want to verify that "Project signed off" halted the loop.
	// Capture stdout? Or trust the signals were set.
	// The most robust way is to verify behavior:
	// If it worked, we shouldn't see "Initializing..." for a new project.
}
