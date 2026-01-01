package runner

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/docker"
	"testing"
)

func TestSession_E2E_DockerFileWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E Docker test in short mode")
	}

	// 1. Setup real Docker client
	dockerCli, err := docker.NewClient("e2e-test")
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}

	// 2. Setup temporary workspace
	tmpWorkspace := t.TempDir()

	// 3. Setup Mock Agent
	mockAgent := agent.NewMockAgent()
	// No need to set response here if we just call ProcessResponse directly,
	// but let's use RunIteration for a full flow.

	s := NewSession(dockerCli, mockAgent, tmpWorkspace, "recac-agent:latest", "e2e-test", 1)
	s.MaxIterations = 1

	ctx := context.Background()

	// 4. Start Session (this should trigger fixPasswdDatabase)
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}
	defer s.Docker.StopContainer(ctx, s.ContainerID)

	// 5. Simulate Agent writing a file
	testFile := "persistence_test.txt"
	testContent := "Verified at " + filepath.Base(tmpWorkspace)
	response := "I will write a file.\n```bash\necho '" + testContent + "' > " + testFile + "\n```"

	_, err = s.ProcessResponse(ctx, response)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	// 6. Verify file exists on HOST
	hostPath := filepath.Join(tmpWorkspace, testFile)
	content, err := os.ReadFile(hostPath)
	if err != nil {
		t.Fatalf("Failed to read file from host workspace: %v", err)
	}

	if string(content) != testContent+"\n" {
		t.Errorf("Content mismatch. Expected '%s', got '%s'", testContent, string(content))
	}

	// 7. Verify sudo works with diagnostics
	diagResponse := "Diagnostics:\n```bash\nid\nwhoami\ngrep appuser /etc/passwd\n```"
	diagOut, _ := s.ProcessResponse(ctx, diagResponse)
	t.Logf("Diagnostic Output:\n%s", diagOut)

	sudoFile := "sudo_test.txt"
	sudoResponse := "I need sudo for this.\n```bash\nsudo touch " + sudoFile + "\n```"
	_, err = s.ProcessResponse(ctx, sudoResponse)
	if err != nil {
		t.Fatalf("Sudo ProcessResponse failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpWorkspace, sudoFile)); os.IsNotExist(err) {
		t.Errorf("Sudo-created file missing on host")
	}
}
