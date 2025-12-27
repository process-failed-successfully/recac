package runner

import (
	"context"
	"strings"
	"testing"
	"recac/internal/pkg/git"
)

type MockManagerAgent struct {
	ReceivedPrompt string
	Response       string
}

func (m *MockManagerAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.ReceivedPrompt = prompt
	return m.Response, nil
}

type MockGitOps struct {
	PushCalled     bool
	PushBranch     string
	CreatePRCalled bool
	CreatePRBranch string
	CreatePRTitle  string
	CreatePRBody   string
}

// Ensure MockGitOps implements git.GitOps interface
var _ git.GitOps = (*MockGitOps)(nil)

func (m *MockGitOps) Push(branch string) error {
	m.PushCalled = true
	m.PushBranch = branch
	return nil
}

func (m *MockGitOps) CreatePR(branch, title, body string) error {
	m.CreatePRCalled = true
	m.CreatePRBranch = branch
	m.CreatePRTitle = title
	m.CreatePRBody = body
	return nil
}

func TestWorkflow_RunCycle_InvokesManager(t *testing.T) {
	// Setup
	mockManager := &MockManagerAgent{Response: "Focus on UI"}
	workflow := NewWorkflow(nil, mockManager)
	
	// Add a failing feature to verify it appears in the prompt
	workflow.Features = []Feature{
		{Category: "ui", Description: "Broken Button", Passes: false},
	}

	// Execute
	feedback, err := workflow.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle failed: %v", err)
	}

	// Verify Manager was invoked
	if feedback != "Focus on UI" {
		t.Errorf("Expected manager feedback 'Focus on UI', got '%s'", feedback)
	}

	// Verify prompt contained QA report
	if !strings.Contains(mockManager.ReceivedPrompt, "QA Report") {
		t.Error("Manager prompt did not contain QA Report")
	}
	if !strings.Contains(mockManager.ReceivedPrompt, "Broken Button") {
		t.Error("Manager prompt did not contain failing feature description")
	}
}

func TestWorkflow_FinishFeature(t *testing.T) {
	mockManager := &MockManagerAgent{Response: "Detailed PR Description"}
	mockGitOps := &MockGitOps{}
	workflow := NewWorkflowWithGitOps(nil, mockManager, mockGitOps)
	
	featureName := "new-button"
	err := workflow.FinishFeature(context.Background(), featureName)
	if err != nil {
		t.Fatalf("FinishFeature failed: %v", err)
	}
	
	// Verify Manager Agent was called to generate PR description
	if !strings.Contains(mockManager.ReceivedPrompt, featureName) {
		t.Errorf("Expected prompt to contain feature name, got %q", mockManager.ReceivedPrompt)
	}
	if !strings.Contains(mockManager.ReceivedPrompt, "PR description") {
		t.Errorf("Expected prompt to contain 'PR description', got %q", mockManager.ReceivedPrompt)
	}
	
	// Verify git.Push was called
	if !mockGitOps.PushCalled {
		t.Error("Expected Push to be called, but it was not")
	}
	expectedBranch := "feature/" + featureName
	if mockGitOps.PushBranch != expectedBranch {
		t.Errorf("Expected Push to be called with branch %q, got %q", expectedBranch, mockGitOps.PushBranch)
	}
	
	// Verify git.CreatePR was called
	if !mockGitOps.CreatePRCalled {
		t.Error("Expected CreatePR to be called, but it was not")
	}
	if mockGitOps.CreatePRBranch != expectedBranch {
		t.Errorf("Expected CreatePR to be called with branch %q, got %q", expectedBranch, mockGitOps.CreatePRBranch)
	}
	expectedTitle := "Implement " + featureName
	if mockGitOps.CreatePRTitle != expectedTitle {
		t.Errorf("Expected CreatePR to be called with title %q, got %q", expectedTitle, mockGitOps.CreatePRTitle)
	}
	if mockGitOps.CreatePRBody != mockManager.Response {
		t.Errorf("Expected CreatePR to be called with body %q (from manager), got %q", mockManager.Response, mockGitOps.CreatePRBody)
	}
}
