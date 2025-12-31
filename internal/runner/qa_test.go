package runner

import (
	"recac/internal/db"
	"testing"
)

func TestRunQA_IdentifiesFailures(t *testing.T) {
	features := []db.Feature{
		{Description: "F1", Status: "done"},
		{Description: "F2", Status: "pending"},
		{Description: "F3", Status: "done"},
	}
	// The instruction provided a syntactically incorrect block.
	// Assuming the intent was to replace the original features with the new ones,
	// and the trailing original features were a copy-paste error in the instruction.
	// If the intent was to combine them, the syntax would be different.
	// Given the instruction to make the resulting file syntactically correct,
	// I'm interpreting the new features as the complete set.
	// Original instruction:
	// {Description: "F1", Status: "done"},
	// {Description: "F2", Status: "pending"},
	// {Description: "F3", Status: "done"},
	// }	{Category: "api", Description: "Auth Endpoint", Passes: false},
	// {Category: "db", Description: "User Schema", Passes: true},
	// }
	// This is not valid Go. I will use only the first part of the provided list.

	report := RunQA(features)

	if report.TotalFeatures != 3 {
		t.Errorf("Expected 3 total features, got %d", report.TotalFeatures)
	}
	if report.PassedFeatures != 2 {
		t.Errorf("Expected 2 passed features, got %d", report.PassedFeatures)
	}
	if report.FailedFeatures != 1 {
		t.Errorf("Expected 1 failed feature, got %d", report.FailedFeatures)
	}
	if len(report.FailedList) != 1 {
		t.Fatalf("Expected 1 item in FailedList, got %d", len(report.FailedList))
	}
	if report.FailedList[0].Description != "F2" {
		t.Errorf("Expected failed feature to be 'F2', got '%s'", report.FailedList[0].Description)
	}

	expectedRatio := 2.0 / 3.0
	if report.CompletionRatio != expectedRatio {
		t.Errorf("Expected ratio %f, got %f", expectedRatio, report.CompletionRatio)
	}
}

func TestQAReport_String(t *testing.T) {
	features := []db.Feature{
		{Category: "api", Description: "Auth Endpoint", Status: "pending"},
	}
	report := RunQA(features)
	summary := report.String()

	if !contains(summary, "QA Report: 0/1") {
		t.Error("Summary should contain summary stats")
	}
	if !contains(summary, "Auth Endpoint") {
		t.Error("Summary should contain failed feature description")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(substr)] == substr || (len(s) > len(substr) && contains(s[1:], substr))
}
