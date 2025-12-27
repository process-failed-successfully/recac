package runner

import (
	"testing"
)

func TestRunQA_IdentifiesFailures(t *testing.T) {
	features := []Feature{
		{Category: "ui", Description: "Login Page", Passes: true},
		{Category: "api", Description: "Auth Endpoint", Passes: false},
		{Category: "db", Description: "User Schema", Passes: true},
	}

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
	if report.FailedList[0].Description != "Auth Endpoint" {
		t.Errorf("Expected failed feature to be 'Auth Endpoint', got '%s'", report.FailedList[0].Description)
	}
	
	expectedRatio := 2.0 / 3.0
	if report.CompletionRatio != expectedRatio {
		t.Errorf("Expected ratio %f, got %f", expectedRatio, report.CompletionRatio)
	}
}

func TestQAReport_String(t *testing.T) {
	features := []Feature{
		{Category: "api", Description: "Auth Endpoint", Passes: false},
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
