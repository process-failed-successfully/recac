package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMockAgent_Initializer_JSON_Structure(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	resp, err := agent.Send(ctx, "You are the Initializer")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Extract JSON from bash heredoc
	// Response is: cat <<EOF > feature_list.json\n{...}\nEOF
	start := strings.Index(resp, "{")
	end := strings.LastIndex(resp, "}")
	if start == -1 || end == -1 {
		t.Fatalf("Could not find JSON object in response: %s", resp)
	}
	jsonStr := resp[start : end+1]

	// The mock response uses shell variable substitution "${RECAC_PROJECT_ID:-PRIMES}".
	// This is valid JSON only if we treat it as a literal string or replace it.
	// However, json.Unmarshal won't like the unescaped "$" inside a string unless it's handled.
	// Actually, inside a string value in JSON, "$" is fine.
	// But `"${RECAC_PROJECT_ID:-PRIMES}"` is the value.
	// Wait, the JSON string in Go code is:
	// "project_name": "${RECAC_PROJECT_ID:-PRIMES}",
	// This is valid JSON string value.

	// Attempt to unmarshal into a generic map to check structure
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to parse extracted JSON: %v\nJSON: %s", err, jsonStr)
	}

	if _, ok := data["project_name"]; !ok {
		t.Error("Missing 'project_name' field")
	}

	features, ok := data["features"].([]interface{})
	if !ok {
		t.Fatal("'features' field is not an array")
	}

	if len(features) == 0 {
		t.Error("Features array is empty")
	}
}
