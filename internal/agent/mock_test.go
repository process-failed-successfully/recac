package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	prompt := "This is a test prompt that is long enough to be truncated"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Response missing prefix, got: %s", response)
	}

	if !strings.Contains(response, "I received your prompt") {
		t.Errorf("Response missing body, got: %s", response)
	}
}

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	t.Run("TPM Prompt returns single task", func(t *testing.T) {
		prompt := "You are a Technical Program Manager. Create tickets."
		response, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)

		var tickets []map[string]interface{}
		err = json.Unmarshal([]byte(response), &tickets)
		assert.NoError(t, err, "Response should be valid JSON")
		assert.Len(t, tickets, 1, "Should return exactly one ticket")

		ticket := tickets[0]
		assert.Contains(t, ticket["title"], "ID:[PRIMES]", "Title should contain ID:[PRIMES]")
		assert.Equal(t, "Task", ticket["type"], "Type should be Task")
	})

	t.Run("Coding Prompt returns correct script (by ID)", func(t *testing.T) {
		prompt := "Implement ID:[PRIMES]. Generate primes.py."
		response, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)

		// Check for bash script structure
		assert.Contains(t, response, "#!/bin/bash", "Should be a bash script")
		assert.Contains(t, response, "cat <<EOF > primes.py", "Should create primes.py")

		// Check for key logic requirements
		assert.Contains(t, response, "import json", "Should import json")
		assert.Contains(t, response, "range(2, n)", "Should iterate up to n (exclusive)")
		assert.Contains(t, response, "json.dump({\"primes\": primes}, f)", "Should dump correct JSON structure")

		// Check for git operations
		assert.Contains(t, response, "python3 primes.py", "Should run the script")
		assert.Contains(t, response, "git add primes.py primes.json", "Should add both files")
		assert.Contains(t, response, "git commit", "Should commit")
		assert.Contains(t, response, "git push", "Should push")
	})

	t.Run("Coding Prompt returns correct script (by text content)", func(t *testing.T) {
		prompt := "Please write a script to generate Prime Numbers and output them."
		response, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)

		assert.Contains(t, response, "#!/bin/bash", "Should be a bash script")
		assert.Contains(t, response, "cat <<EOF > primes.py", "Should create primes.py")
		assert.Contains(t, response, "json.dump({\"primes\": primes}, f)", "Should dump correct JSON structure")
	})

	t.Run("Coding Prompt with lowercase keywords (smoke test)", func(t *testing.T) {
		prompt := "Description: Must generate primes up to 10000"
		response, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)

		// This should trigger the coding heuristic
		assert.Contains(t, response, "#!/bin/bash", "Should be a bash script")
		assert.Contains(t, response, "cat <<EOF > primes.py", "Should create primes.py")
	})
}

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}
