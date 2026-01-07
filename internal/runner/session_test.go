package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/docker"
	"recac/internal/git"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

type MockAgent struct {
	Response string
}

func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if onChunk != nil {
		onChunk(m.Response)
	}
	return m.Response, nil
}

func TestSession_ReadSpec(t *testing.T) {
	tmpDir := t.TempDir()
	specContent := "Application Specification v1.0"
	specPath := filepath.Join(tmpDir, "app_spec.txt")

	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	session := NewSession(nil, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)

	content, err := session.ReadSpec()
	if err != nil {
		t.Fatalf("ReadSpec failed: %v", err)
	}

	if content != specContent {
		t.Errorf("Expected spec content '%s', got '%s'", specContent, content)
	}
}

func TestSession_ReadSpec_Missing(t *testing.T) {
	tmpDir := t.TempDir()
	session := NewSession(nil, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)

	_, err := session.ReadSpec()
	if err == nil {
		t.Error("Expected error for missing spec file, got nil")
	}
}

func TestSession_AgentReadsSpec(t *testing.T) {
	tmpDir := t.TempDir()
	specContent := "Application Specification\nThis is a test specification file for verification."
	specPath := filepath.Join(tmpDir, "app_spec.txt")

	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	mockDocker, _ := docker.NewMockClient()
	session := NewSession(mockDocker, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)

	spec, err := session.ReadSpec()
	if err != nil {
		t.Fatalf("ReadSpec failed: %v", err)
	}
	if spec != specContent {
		t.Errorf("Expected spec content '%s', got '%s'", specContent, spec)
	}

	ctx := context.Background()
	err = session.Start(ctx)
	if err != nil {
		if err.Error() == "failed to read spec file" ||
			err.Error() == "Warning: Failed to read spec" {
			t.Fatalf("Start() failed due to ReadSpec error: %v", err)
		}
	}

	expectedLength := len(specContent)
	if len(spec) != expectedLength {
		t.Errorf("Spec length mismatch: expected %d, got %d", expectedLength, len(spec))
	}
}

func TestSession_Start_PassesUser(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "app_spec.txt")
	os.WriteFile(specPath, []byte("test"), 0644)

	d, mock := docker.NewMockClient()
	passedUser := ""
	mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
		passedUser = config.User
		return container.CreateResponse{ID: "test"}, nil
	}

	session := NewSession(d, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if passedUser == "" {
		t.Error("Expected non-empty user to be passed to Docker")
	}
}

func TestSession_SyncFeatureFile_RetryOnPermissionDenied(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "feature_list.json")

	// Create a file and make it read-only
	os.WriteFile(path, []byte("old"), 0444)

	s := &Session{
		Workspace: tmpDir,
		Notifier:  notify.NewManager(func(string, ...interface{}) {}),
		Logger:    telemetry.NewLogger(true, ""),
	}

	fl := db.FeatureList{
		ProjectName: "Test",
		Features:    []db.Feature{{ID: "1", Status: "done"}},
	}

	// This should successfully overwrite because of os.Remove fallback
	s.syncFeatureFile(fl)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file after sync: %v", err)
	}

	if !strings.Contains(string(data), "done") {
		t.Errorf("File content doesn't match expected after sync recovery: %s", string(data))
	}
}

func TestSession_SelectPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	specContent := "Test Spec"
	specPath := filepath.Join(tmpDir, "app_spec.txt")
	os.WriteFile(specPath, []byte(specContent), 0644)

	session := NewSession(nil, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.ManagerFrequency = 3

	// Session 1: Initializer
	session.Iteration = 1
	prompt, _, isManager, err := session.SelectPrompt()
	if err != nil {
		t.Fatalf("SelectPrompt failed: %v", err)
	}
	if isManager {
		t.Error("Iteration 1 should not be manager")
	}
	if !strings.Contains(prompt, "INITIALIZER") {
		t.Errorf("Expected INITIALIZER prompt, got %q", prompt)
	}

	// Create valid feature list (non-empty) so we exit Initializer mode
	featureContent := `{"project_name": "Test", "features": [{"id":"1", "description":"test", "status":"pending"}]}`
	os.WriteFile(filepath.Join(tmpDir, "feature_list.json"), []byte(featureContent), 0644)

	// Session 2: Coding Agent
	session.Iteration = 2
	prompt, _, isManager, err = session.SelectPrompt()
	if err != nil {
		t.Fatalf("SelectPrompt failed: %v", err)
	}
	if isManager {
		t.Error("Iteration 2 should not be manager")
	}
	if !strings.Contains(prompt, "CODING AGENT") {
		t.Errorf("Expected CODING AGENT prompt, got %q", prompt)
	}

	// Session 3: Manager Review (frequency)
	session.Iteration = 3
	prompt, _, isManager, err = session.SelectPrompt()
	if err != nil {
		t.Fatalf("SelectPrompt failed: %v", err)
	}
	if !isManager {
		t.Error("Iteration 3 should be manager")
	}
	if !strings.Contains(prompt, "PROJECT MANAGER") {
		t.Errorf("Expected Manager prompt, got %q", prompt)
	}

	// Session 4: Coding Agent
	session.Iteration = 4
	prompt, _, _, _ = session.SelectPrompt()
	if !strings.Contains(prompt, "CODING AGENT") {
		t.Errorf("Expected CODING AGENT prompt, got %q", prompt)
	}

	// Session 5: Manager Review (signal)
	session.createSignal("TRIGGER_MANAGER")
	session.Iteration = 5
	prompt, _, isManager, err = session.SelectPrompt()
	if err != nil {
		t.Fatalf("SelectPrompt failed: %v", err)
	}
	if !isManager {
		t.Error("TRIGGER_MANAGER should trigger manager")
	}

}

func TestSession_AgentStatePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, ".agent_state.json")

	session := NewSessionWithStateFile(nil, &MockAgent{}, tmpDir, "alpine", "test-project", stateFile, "gemini", "gemini-pro", 1)

	// Initialize
	if err := session.InitializeAgentState(1000); err != nil {
		t.Fatalf("Failed to init state: %v", err)
	}

	// Modify state manually to verify save
	state, _ := session.StateManager.Load()
	state.CurrentTokens = 500
	session.StateManager.Save(state)

	// Save via session
	if err := session.SaveAgentState(); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Load via session
	if err := session.LoadAgentState(); err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Verify loaded
	loadedState, _ := session.StateManager.Load()
	if loadedState.CurrentTokens != 500 {
		t.Errorf("Expected 500 tokens, got %d", loadedState.CurrentTokens)
	}
}

func TestSession_Signals(t *testing.T) {
	tmpDir := t.TempDir()
	session := NewSession(nil, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)

	// Test createSignal
	if err := session.createSignal("TEST_SIGNAL"); err != nil {
		t.Fatalf("Failed to create signal: %v", err)
	}

	// Test hasSignal
	if !session.hasSignal("TEST_SIGNAL") {
		t.Error("Expected hasSignal to return true")
	}

	// Test clearSignal
	session.clearSignal("TEST_SIGNAL")
	if session.hasSignal("TEST_SIGNAL") {
		t.Error("Expected signal to be cleared")
	}

	// Test checkCompletion
	if session.checkCompletion() {
		t.Error("Expected not completed")
	}
	session.createSignal("COMPLETED")
	if !session.checkCompletion() {
		t.Error("Expected completed")
	}
}

func TestSession_Stop(t *testing.T) {
	mockDocker, _ := docker.NewMockClient()
	session := NewSession(mockDocker, &MockAgent{}, "/tmp", "alpine", "test-project", "gemini", "gemini-pro", 1)

	ctx := context.Background()

	// Stop without container ID
	if err := session.Stop(ctx); err != nil {
		t.Errorf("Stop failed with empty ID: %v", err)
	}

	// Stop with container ID
	session.ContainerID = "test-container"
	if err := session.Stop(ctx); err != nil {
		t.Errorf("Stop failed with valid ID: %v", err)
	}

	if session.ContainerID != "" {
		t.Error("Expected ContainerID to be cleared")
	}
}

func TestSession_RunCleanerAgent(t *testing.T) {
	tmpDir := t.TempDir()
	session := NewSession(nil, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)

	// Create temp file to delete
	tmpFile := filepath.Join(tmpDir, "to_delete.txt")
	os.WriteFile(tmpFile, []byte("data"), 0644)

	// Create temp_files.txt listing it
	listFile := filepath.Join(tmpDir, "temp_files.txt")
	os.WriteFile(listFile, []byte("to_delete.txt\n# comment\n\n"), 0644)

	if err := session.runCleanerAgent(context.Background()); err != nil {
		t.Fatalf("Cleaner agent failed: %v", err)
	}

	// Verify deleted
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("File should have been deleted")
	}

	// Verify list file deleted
	if _, err := os.Stat(listFile); !os.IsNotExist(err) {
		t.Error("temp_files.txt should have been deleted")
	}
}

func TestSession_LoadFeatures(t *testing.T) {
	tmpDir := t.TempDir()
	session := NewSession(nil, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)

	features := session.loadFeatures()
	if features != nil {
		t.Error("Expected nil features when file missing")
	}

	// Write feature list
	listPath := filepath.Join(tmpDir, "feature_list.json")
	content := `{"project_name": "Test", "features": [{"id":"1", "description":"feat 1", "status":"done"}]}`
	os.WriteFile(listPath, []byte(content), 0644)

	features = session.loadFeatures()
	if len(features) != 1 {
		t.Errorf("Expected 1 feature, got %d", len(features))
	}
	if features[0].Description != "feat 1" {
		t.Errorf("Expected feat 1, got %s", features[0].Description)
	}
}

func TestSession_RunLoop_SingleIteration(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	mockDocker, _ := docker.NewMockClient()
	session := NewSession(mockDocker, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.MaxIterations = 1

	ctx := context.Background()
	if err := session.RunLoop(ctx); err != nil && err != ErrMaxIterations {
		t.Fatalf("RunLoop failed: %v", err)
	}
}

func TestSession_RunQAAgent(t *testing.T) {
	tmpDir := t.TempDir()
	mockAgent := &MockAgentForQA{
		Response:  "PASS",
		Workspace: tmpDir,
	}
	session := NewSession(nil, mockAgent, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.QAAgent = mockAgent

	// Create feature list (all passing)
	listPath := filepath.Join(tmpDir, "feature_list.json")
	content := `{"project_name": "Test", "features": [{"id":"1", "description":"feat 1", "status":"done"}]}`
	os.WriteFile(listPath, []byte(content), 0644)

	if err := session.runQAAgent(context.Background()); err != nil {
		t.Errorf("Expected QA to pass: %v", err)
	}

	// Create feature list (failing)
	content = `{"project_name": "Test", "features": [{"id":"1", "description":"feat 1", "status":"pending"}]}`
	os.WriteFile(listPath, []byte(content), 0644)
	mockAgent.Response = "FAIL"

	if err := session.runQAAgent(context.Background()); err == nil {
		t.Error("Expected QA to fail")
	}
}

func TestSession_RunManagerAgent(t *testing.T) {
	tmpDir := t.TempDir()
	mockAgent := &MockAgent{}
	session := NewSession(nil, mockAgent, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.ManagerAgent = mockAgent

	// 1. All passing -> Approved
	content := `{"project_name": "Test", "features": [{"id":"1", "description":"feat 1", "status":"done"}]}`
	os.WriteFile(filepath.Join(tmpDir, "feature_list.json"), []byte(content), 0644)
	if session.DBStore != nil {
		_ = session.DBStore.SaveFeatures(content)
	}
	if err := session.runManagerAgent(context.Background()); err != nil {
		t.Errorf("Expected manager approval, got error: %v", err)
	}

	// 2. Failing -> Rejected
	content = `{"project_name": "Test", "features": [{"id":"1", "description":"feat 1", "status":"pending"}]}`
	os.WriteFile(filepath.Join(tmpDir, "feature_list.json"), []byte(content), 0644)
	if session.DBStore != nil {
		_ = session.DBStore.SaveFeatures(content)
	}
	if err := session.runManagerAgent(context.Background()); err == nil {
		t.Error("Expected manager rejection, got nil")
	}
}

func TestMin(t *testing.T) {
	if min(1, 2) != 1 {
		t.Error("min(1,2) != 1")
	}
	if min(2, 1) != 1 {
		t.Error("min(2,1) != 1")
	}
	if min(1, 1) != 1 {
		t.Error("min(1,1) != 1")
	}
}

func TestSession_Start_MountsBridge(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "app_spec.txt")
	os.WriteFile(specPath, []byte("test"), 0644)

	// Create a dummy agent-bridge in the current directory (since test runs in package dir)
	cwd, _ := os.Getwd()
	bridgePath := filepath.Join(cwd, "agent-bridge")
	if _, err := os.Stat(bridgePath); os.IsNotExist(err) {
		os.WriteFile(bridgePath, []byte("dummy"), 0755)
		defer os.Remove(bridgePath)
	}

	d, mock := docker.NewMockClient()
	var mountedBinds []string
	mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
		mountedBinds = hostConfig.Binds
		return container.CreateResponse{ID: "test"}, nil
	}

	session := NewSession(d, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	foundBridge := false
	for _, bind := range mountedBinds {
		if strings.Contains(bind, "/usr/local/bin/agent-bridge:ro") {
			foundBridge = true
			break
		}
	}

	// If the bridge was found in standard location, it won't be in mountedBinds
	if bridge, _ := session.findAgentBridgeBinary(); bridge == "/usr/local/bin/agent-bridge" {
		t.Log("Agent bridge found in standard location, skipping mount check")
		return
	}

	if !foundBridge {
		t.Errorf("Expected agent-bridge bind mount in %v", mountedBinds)
	}
}
func TestSession_FixPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	d, _ := docker.NewMockClient()
	session := NewSession(d, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.ContainerID = "test-container"

	// fixPermissions calls ExecAsUser, which in MockClient returns mock-exec-id and nil error by default.
	// It doesn't need complex mocking if we just want to verify it doesn't crash/error on happy path.
	if err := session.fixPermissions(context.Background()); err != nil {
		t.Errorf("fixPermissions failed: %v", err)
	}
}

func TestSession_EnsureConflictTask(t *testing.T) {
	tmpDir := t.TempDir()
	session := NewSession(nil, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)

	// Case 1: Add new conflict task
	listPath := filepath.Join(tmpDir, "feature_list.json")
	content := `{"project_name": "Test", "features": [{"id":"1", "description":"feat 1", "status":"done"}]}`
	os.WriteFile(listPath, []byte(content), 0644)

	session.EnsureConflictTask()

	data, _ := os.ReadFile(listPath)
	if !strings.Contains(string(data), "CONFLICT_RES") {
		t.Errorf("Expected CONFLICT_RES task in feature list, got %s", string(data))
	}

	// Case 2: Conflict task exists and is done, should be reset to todo
	content = `{"project_name": "Test", "features": [{"id":"CONFLICT_RES", "description":"conflict", "status":"done", "passes":true}]}`
	os.WriteFile(listPath, []byte(content), 0644)

	session.EnsureConflictTask()

	data, _ = os.ReadFile(listPath)
	if !strings.Contains(string(data), `"status": "todo"`) {
		t.Errorf("Expected CONFLICT_RES task to be reset to todo, got %s", string(data))
	}
}

func TestSession_PushProgress(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize git repo
	exec.Command("git", "init", tmpDir).Run()
	gitClient := git.NewClient()
	gitClient.Config(tmpDir, "user.email", "test@example.com")
	gitClient.Config(tmpDir, "user.name", "Test User")

	// Create initial commit on main
	os.WriteFile(filepath.Join(tmpDir, "initial.txt"), []byte("init"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Create feature branch
	exec.Command("git", "-C", tmpDir, "checkout", "-b", "agent/test-branch").Run()

	// Set up a "remote"
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", tmpDir, "remote", "add", "origin", remoteDir).Run()

	session := &Session{
		Workspace: tmpDir,
		Logger:    telemetry.NewLogger(true, ""),
		Iteration: 1,
	}

	// Modify a file
	os.WriteFile(filepath.Join(tmpDir, "progress.txt"), []byte("work in progress"), 0644)

	// Call pushProgress
	session.pushProgress(context.Background())

	// Verify commit exists on branch
	out, _ := exec.Command("git", "-C", tmpDir, "log", "-1", "--oneline").Output()
	if !strings.Contains(string(out), "chore: progress update") {
		t.Errorf("Expected progress commit, got: %s", string(out))
	}

	// Verify pushed to remote
	remoteOut, _ := exec.Command("git", "-C", remoteDir, "branch").Output()
	if !strings.Contains(string(remoteOut), "agent/test-branch") {
		t.Errorf("Expected branch to be pushed to remote, got: %s", string(remoteOut))
	}
}
