package agent

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestMockAgent_Coding_IDExtraction(t *testing.T) {
	agent := NewMockAgent()

	// Case 1: Prompt with brackets
	prompt1 := "Implement feature [MFLP-1234] primes.py"
	response1, _ := agent.Send(context.Background(), prompt1)
	if !strings.Contains(response1, "git checkout -B agent/MFLP-1234") {
		t.Errorf("Failed to extract ID from prompt brackets. Response: %s", response1)
	}

	// Case 2: Environment variable override
	os.Setenv("RECAC_PROJECT_ID", "ENV-ID-999")
	defer os.Unsetenv("RECAC_PROJECT_ID")

	prompt2 := "Implement feature [MFLP-1234] primes.py" // Even with prompt ID
	response2, _ := agent.Send(context.Background(), prompt2)
	if !strings.Contains(response2, "git checkout -B agent/ENV-ID-999") {
		t.Errorf("Failed to prioritize environment variable. Response: %s", response2)
	}

	// Case 3: Fallback when no ID and no Env
	os.Unsetenv("RECAC_PROJECT_ID")
	prompt3 := "Implement primes.py"
	response3, _ := agent.Send(context.Background(), prompt3)
	if !strings.Contains(response3, "git checkout -B agent/PRIMES-mock") {
		t.Errorf("Failed to fallback to default. Response: %s", response3)
	}
}
