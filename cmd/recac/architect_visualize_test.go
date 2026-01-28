package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestArchitectVisualize(t *testing.T) {
	// 1. Setup Temp Dir
	tempDir, err := os.MkdirTemp("", "recac-arch-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 2. Create architecture.yaml
	archContent := `
version: "1.0"
system_name: "TestSystem"
components:
  - id: "api-gateway"
    type: "service"
    consumes: []
    produces:
      - target: "user-service"
        event: "UserCreated"
  - id: "user-service"
    type: "service"
    consumes:
      - source: "api-gateway"
        type: "UserCreated"
    produces:
      - target: "db"
  - id: "db"
    type: "database"
  - id: "weird \"component\""
    type: "worker"
`
	if err := os.WriteFile(filepath.Join(tempDir, "architecture.yaml"), []byte(archContent), 0644); err != nil {
		t.Fatalf("Failed to write architecture.yaml: %v", err)
	}

	// 3. Setup Command
	cmd := &cobra.Command{
		Use: "visualize",
		RunE: runArchitectVisualize,
	}
	cmd.Flags().String("dir", "", "Directory")

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--dir", tempDir})

	// 4. Execute
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	// 5. Assert
	output := buf.String()
	t.Logf("Output:\n%s", output)

	if !strings.Contains(output, "graph TD") {
		t.Errorf("Output missing 'graph TD'")
	}
	if !strings.Contains(output, "api_gateway[\"api-gateway\\n(service)\"]") {
		t.Errorf("Output missing api-gateway node")
	}

	// Check deduplicated edge
	if strings.Count(output, "api_gateway -- UserCreated --> user_service") != 1 {
		t.Errorf("Edge 'UserCreated' should appear exactly once")
	}

	// Check escaping
	// weird "component" -> weird_component (quotes stripped by safeMermaidID)
	if !strings.Contains(output, "weird_component[\"weird 'component'\\n(worker)\"]") {
		t.Errorf("Output missing weird component node or escaping failed")
	}
}
