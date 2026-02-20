package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type MockPlanAgent struct {
	Response string
}

func (m *MockPlanAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockPlanAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if onChunk != nil {
		onChunk(m.Response)
	}
	return m.Response, nil
}

func TestSession_GeneratePlanOnly(t *testing.T) {
	tmpDir := t.TempDir()
	specContent := "Application Specification v1.0"
	specPath := filepath.Join(tmpDir, "app_spec.txt")

	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	expectedPlan := "# Implementation Plan\n\n1. Step 1\n2. Step 2"
	mockAgent := &MockPlanAgent{Response: expectedPlan}

	session := NewSession(nil, mockAgent, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.PlanOnly = true

	// Test GeneratePlanOnly directly
	err := session.GeneratePlanOnly(context.Background())
	if err != nil {
		t.Fatalf("GeneratePlanOnly failed: %v", err)
	}

	// Verify PLAN.md content
	planPath := filepath.Join(tmpDir, "PLAN.md")
	content, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("Failed to read PLAN.md: %v", err)
	}

	if string(content) != expectedPlan {
		t.Errorf("Expected plan content '%s', got '%s'", expectedPlan, string(content))
	}
}

func TestSession_RunLoop_PlanOnly(t *testing.T) {
	tmpDir := t.TempDir()
	specContent := "Application Specification v1.0"
	specPath := filepath.Join(tmpDir, "app_spec.txt")

	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	expectedPlan := "# Implementation Plan\n\n1. Step 1\n2. Step 2"
	mockAgent := &MockPlanAgent{Response: expectedPlan}

	session := NewSession(nil, mockAgent, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.PlanOnly = true

	// Test RunLoop delegates to GeneratePlanOnly
	err := session.RunLoop(context.Background())
	if err != nil {
		t.Fatalf("RunLoop failed: %v", err)
	}

	// Verify PLAN.md content
	planPath := filepath.Join(tmpDir, "PLAN.md")
	content, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("Failed to read PLAN.md: %v", err)
	}

	if string(content) != expectedPlan {
		t.Errorf("Expected plan content '%s', got '%s'", expectedPlan, string(content))
	}
}
