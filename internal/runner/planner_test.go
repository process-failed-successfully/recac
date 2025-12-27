package runner

import (
	"context"
	"testing"
)

type MockPlannerAgent struct {
	Response string
	Err      error
}

func (m *MockPlannerAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, m.Err
}

func TestGenerateFeatureList_Success(t *testing.T) {
	mockResponse := `[
		{
			"category": "cli",
			"description": "Verify version command",
			"steps": ["Run version"],
			"passes": false
		}
	]`
	
mockAgent := &MockPlannerAgent{Response: mockResponse}
	
	features, err := GenerateFeatureList(context.Background(), mockAgent, "Spec content")
	if err != nil {
		t.Fatalf("GenerateFeatureList failed: %v", err)
	}
	
	if len(features) != 1 {
		t.Errorf("Expected 1 feature, got %d", len(features))
	}
	if features[0].Category != "cli" {
		t.Errorf("Expected category 'cli', got '%s'", features[0].Category)
	}
}

func TestGenerateFeatureList_HandlesMarkdown(t *testing.T) {
	mockResponse := "```json\n" + `[
		{
			"category": "ui",
			"description": "Verify UI",
			"steps": ["Check UI"],
			"passes": false
		}
	]` + "\n```"
	
mockAgent := &MockPlannerAgent{Response: mockResponse}
	
	features, err := GenerateFeatureList(context.Background(), mockAgent, "Spec content")
	if err != nil {
		t.Fatalf("GenerateFeatureList failed: %v", err)
	}
	
	if len(features) != 1 {
		t.Errorf("Expected 1 feature, got %d", len(features))
	}
}

