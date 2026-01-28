package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestPlanVisualize(t *testing.T) {
	// 1. Create temporary feature list
	content := `{
  "project_name": "Test Project",
  "features": [
    {
      "id": "F-1",
      "description": "Feature 1",
      "status": "done",
      "dependencies": {
        "depends_on_ids": []
      }
    },
    {
      "id": "F-2",
      "description": "Feature 2 with a very long description that should be truncated",
      "status": "pending",
      "dependencies": {
        "depends_on_ids": ["F-1"]
      }
    }
  ]
}`
	tmpFile, err := os.CreateTemp("", "feature_list_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// 2. Setup Command
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// 3. Run
	err = runPlanVisualize(cmd, []string{tmpFile.Name()})
	if err != nil {
		t.Fatalf("runPlanVisualize failed: %v", err)
	}

	// 4. Assert
	output := buf.String()
	t.Logf("Output:\n%s", output)

	if !strings.Contains(output, "graph TD") {
		t.Errorf("Expected 'graph TD', got %s", output)
	}

	// Check Node F-1 (sanitized to F_1)
	if !strings.Contains(output, "F_1[\"Feature 1\"]:::done") {
		t.Errorf("Expected Feature 1 node, got %s", output)
	}

	// Check Node F-2 (sanitized to F_2) and truncation
	// "Feature 2 with a very long de..." (27 chars + ...)
	expectedDesc := "Feature 2 with a very long ..."
	if !strings.Contains(output, "F_2[\"" + expectedDesc + "\"]:::pending") {
		t.Errorf("Expected Feature 2 node with truncated description, got %s", output)
	}

	// Check Edge (F-1 is dep of F-2, so F_1 --> F_2)
	if !strings.Contains(output, "F_1 --> F_2") {
		t.Errorf("Expected dependency edge F_1 --> F_2, got %s", output)
	}
}
