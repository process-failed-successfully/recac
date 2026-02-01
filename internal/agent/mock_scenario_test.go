package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMockAgent_TPM(t *testing.T) {
	agent := NewMockAgent()

	prompt := `You are an expert Technical Program Manager (TPM)...
### Application Specification:
### ID:[PRIMES] Prime Number Script
...
`
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	t.Logf("Response: %s", response)

	if !strings.Contains(response, "ID:[PRIMES]") {
		t.Errorf("Response missing extracted ID 'ID:[PRIMES]'")
	}

	// Verify JSON validity
	var tickets []map[string]interface{}
	if err := json.Unmarshal([]byte(response), &tickets); err != nil {
		t.Errorf("Response is not valid JSON: %v", err)
	}
}

func TestMockAgent_Coding(t *testing.T) {
	agent := NewMockAgent()

	prompt := "Implement a python script named 'primes.py'..."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Response missing primes.py creation script")
	}
}
