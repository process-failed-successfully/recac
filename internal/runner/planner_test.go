package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type MockPlannerAgent struct {
	Response string
	Err      error
}

func (m *MockPlannerAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, m.Err
}

func (m *MockPlannerAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if onChunk != nil {
		onChunk(m.Response)
	}
	return m.Response, m.Err
}

func TestGenerateFeatureList(t *testing.T) {
	ctx := context.Background()

	// Test case 1: Agent returns direct JSON
	mockResponse := `{
		"project_name": "Test Project",
		"features": [
			{
				"category": "core",
				"description": "Verify core functionality",
				"steps": ["Run core tests"],
				"status": "pending"
			},
			{
				"category": "cli",
				"description": "Verify CLI functionality",
				"steps": ["Run CLI tests"],
				"status": "pending"
			}
		]
	}`
	mockAgent := &MockPlannerAgent{Response: mockResponse}

	featureList, err := GenerateFeatureList(ctx, mockAgent, "Spec content")
	if err != nil {
		t.Fatalf("GenerateFeatureList failed: %v", err)
	}

	if len(featureList.Features) != 2 {
		t.Errorf("Expected 2 features, got %d", len(featureList.Features))
	}

	if featureList.Features[0].Category != "core" {
		t.Errorf("Expected category 'core', got '%s'", featureList.Features[0].Category)
	}

	// Test case 2: Agent returns markdown block
	mockAgent.Response = "Here is the plan:\n```json\n{\n  \"project_name\": \"Test\",\n  \"features\": [\n    {\n      \"category\": \"ui\",\n      \"description\": \"Login Page\",\n      \"steps\": [\"Create HTML\"],\n      \"status\": \"pending\"\n    }\n  ]\n}\n```"

	featureList, err = GenerateFeatureList(ctx, mockAgent, "Build a login page")
	if err != nil {
		t.Fatalf("GenerateFeatureList failed with markdown: %v", err)
	}

	if len(featureList.Features) != 1 {
		t.Errorf("Expected 1 feature, got %d", len(featureList.Features))
	}

	if featureList.Features[0].Description != "Login Page" {
		t.Errorf("Expected description 'Login Page', got '%s'", featureList.Features[0].Description)
	}
}

func TestGeneratePlanOnly(t *testing.T) {
	ctx := context.Background()
	mockPlan := "# Implementation Plan\n\n- [ ] Step 1"
	mockAgent := &MockPlannerAgent{Response: mockPlan}

	// Create temp workspace
	workspace := t.TempDir()
	specPath := filepath.Join(workspace, "app_spec.txt")
	if err := os.WriteFile(specPath, []byte("Do something"), 0644); err != nil {
		t.Fatal(err)
	}

	session := &Session{
		Agent:     mockAgent,
		Workspace: workspace,
		SpecFile:  "app_spec.txt",
	}

	if err := GeneratePlanOnly(ctx, session); err != nil {
		t.Fatalf("GeneratePlanOnly failed: %v", err)
	}

	// Verify PLAN.md
	planPath := filepath.Join(workspace, "PLAN.md")
	content, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("Failed to read PLAN.md: %v", err)
	}

	if string(content) != mockPlan {
		t.Errorf("Expected plan '%s', got '%s'", mockPlan, string(content))
	}
}
