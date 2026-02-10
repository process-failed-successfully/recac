package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentDockerfile_Consistency(t *testing.T) {
	// Find root path
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// We need to find the root where Dockerfile resides.
	rootPath := findRoot(wd)
	if rootPath == "" {
		t.Skip("Could not find project root, skipping consistency test")
	}

	rootDockerfileBytes, err := os.ReadFile(filepath.Join(rootPath, "Dockerfile"))
	if err != nil {
		t.Fatalf("Failed to read root Dockerfile: %v", err)
	}
	rootDockerfile := string(rootDockerfileBytes)

	// DefaultAgentDockerfile is populated via //go:embed agent.Dockerfile in client.go
	agentDockerfile := DefaultAgentDockerfile

	// Critical tools that MUST be consistent if present in root Dockerfile
	tools := []string{
		"npm install -g @google/gemini-cli",
		"npm install -g opencode-ai",
		"curl -fsS https://cursor.com/install | bash",
	}

	for _, tool := range tools {
		if strings.Contains(rootDockerfile, tool) {
			if !strings.Contains(agentDockerfile, tool) {
				t.Errorf("Agent Dockerfile is inconsistent! Missing tool command found in root Dockerfile: %q", tool)
			}
		}
	}
}

func findRoot(start string) string {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
