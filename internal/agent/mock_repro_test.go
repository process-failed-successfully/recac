package agent

import (
	"context"
	"strings"
	"testing"
)

// TestMockAgent_Initializer_Script ensures that the generated script
// correctly pipes the file to agent-bridge import, rather than using a flag.
func TestMockAgent_Initializer_Script(t *testing.T) {
	agent := NewMockAgent()

	// Prompt that triggers the Initializer logic
	prompt := "Initialize the project with feature_list.json"

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Agent failed: %v", err)
	}

	// 1. Verify it generates the feature_list.json file
	if !strings.Contains(response, "cat << 'EOF' > feature_list.json") {
		t.Error("Script does not create feature_list.json")
	}

	// 2. Verify it uses pipe logic `cat feature_list.json | agent-bridge import`
	// OR `agent-bridge import < feature_list.json`
	// It MUST NOT use `--file` which is unsupported.
	if strings.Contains(response, "agent-bridge import --file") {
		t.Error("Script uses unsupported '--file' flag for agent-bridge import. Should use stdin redirection.")
	}

	if !strings.Contains(response, "agent-bridge import") {
		t.Error("Script does not call agent-bridge import")
	}
}
