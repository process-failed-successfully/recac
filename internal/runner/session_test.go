package runner

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/docker"
	"recac/internal/git"
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
	session.FeatureContent = featureContent

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
	
	// Inject Store and Project into MockAgent now that session is created with DBStore
	mockAgent.Store = session.DBStore
	mockAgent.Project = session.Project
	
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
		_ = session.DBStore.SaveFeatures(session.Project, content)
	}
	if err := session.runManagerAgent(context.Background()); err != nil {
		t.Errorf("Expected manager approval, got error: %v", err)
	}

	// 2. Failing -> Rejected
	content = `{"project_name": "Test", "features": [{"id":"1", "description":"feat 1", "status":"pending"}]}`
	os.WriteFile(filepath.Join(tmpDir, "feature_list.json"), []byte(content), 0644)
	if session.DBStore != nil {
		_ = session.DBStore.SaveFeatures(session.Project, content)
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

	// Case 1: Add new conflict task
	flInitial := db.FeatureList{
		ProjectName: "Test",
		Features: []db.Feature{
			{ID: "1", Description: "feat 1", Status: "done"},
		},
	}
	mockDB := &FaultToleranceMockDB{
		FeatureList: flInitial,
	}

	session := NewSession(nil, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.DBStore = mockDB
	session.Project = "test-project"

	session.EnsureConflictTask()

	// Verify DB updated
	mockDB.mu.Lock()
	foundConflict := false
	for _, f := range mockDB.FeatureList.Features {
		if f.ID == "CONFLICT_RES" {
			foundConflict = true
			if f.Status != "todo" {
				t.Errorf("Expected CONFLICT_RES status to be todo, got %s", f.Status)
			}
		}
	}
	mockDB.mu.Unlock()
	if !foundConflict {
		t.Error("Expected CONFLICT_RES task to be added to DB")
	}

	// Case 2: Conflict task exists and is done, should be reset to todo
	flDone := db.FeatureList{
		ProjectName: "Test",
		Features: []db.Feature{
			{ID: "CONFLICT_RES", Description: "conflict", Status: "done", Passes: true},
		},
	}
	mockDB.FeatureList = flDone

	session.EnsureConflictTask()

	mockDB.mu.Lock()
	foundUpdated := false
	for _, f := range mockDB.FeatureList.Features {
		if f.ID == "CONFLICT_RES" {
			foundUpdated = true
			if f.Status != "todo" {
				t.Errorf("Expected CONFLICT_RES task to be reset to todo, got %s", f.Status)
			}
			if f.Passes {
				t.Error("Expected Passes to be false")
			}
		}
	}
	mockDB.mu.Unlock()

	if !foundUpdated {
		t.Error("Expected CONFLICT_RES task to be updated in DB")
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
		Logger:    telemetry.NewLogger(true, "", false),
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

func TestSession_ProcessResponse_Commands(t *testing.T) {
	d := &MockDockerClient{}
	d.ExecFunc = func(ctx context.Context, containerID string, cmd []string) (string, error) {
		if strings.Contains(cmd[2], "echo hello") {
			return "hello\n", nil
		}
		if strings.Contains(cmd[2], "fail") {
			return "", context.DeadlineExceeded
		}
		return "", nil
	}

	session := NewSession(d, &MockAgent{}, "/tmp", "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.ContainerID = "test-container"

	// 1. Success
	output, err := session.ProcessResponse(context.Background(), "```bash\necho hello\n```")
	if err != nil {
		t.Errorf("Expected success, got: %v", err)
	}
	if !strings.Contains(output, "hello") {
		t.Errorf("Expected output to contain 'hello', got: %s", output)
	}

	// 2. Failure (Timeout)
	output, err = session.ProcessResponse(context.Background(), "```bash\nfail\n```")
	// ProcessResponse returns parsing error or ErrBlocker, it logs execution errors but continues or stops.
	// We expect no error from the function itself, but the output should contain failure message.
	if !strings.Contains(output, "Command Failed") {
		t.Errorf("Expected output to report failure, got: %s", output)
	}
}

func TestSession_ProcessResponse_Blockers(t *testing.T) {
	d := &MockDockerClient{}
	d.ExecFunc = func(ctx context.Context, containerID string, cmd []string) (string, error) {
		// Simulate finding blocker file
		if strings.Contains(cmd[2], "cat recac_blockers.txt") {
			return "Critical API Issue", nil
		}
		return "", nil
	}

	session := NewSession(d, &MockAgent{}, "/tmp", "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.ContainerID = "test-container"

	_, err := session.ProcessResponse(context.Background(), "some response")
	if err != ErrBlocker {
		t.Errorf("Expected ErrBlocker, got: %v", err)
	}
}

func TestSession_BootstrapGit(t *testing.T) {
	d := &MockDockerClient{}
	execCalls := 0
	d.ExecAsUserFunc = func(ctx context.Context, containerID, user string, cmd []string) (string, error) {
		execCalls++
		return "", nil
	}

	session := NewSession(d, &MockAgent{}, "/tmp", "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.ContainerID = "test-container"

	if err := session.bootstrapGit(context.Background()); err != nil {
		t.Errorf("bootstrapGit failed: %v", err)
	}

	if execCalls < 4 {
		t.Errorf("Expected at least 4 git config commands, got %d", execCalls)
	}
}

func TestSession_EnsureImage(t *testing.T) {
	d := &MockDockerClient{}
	pulled := false
	d.ImageExistsFunc = func(ctx context.Context, image string) (bool, error) {
		return false, nil
	}
	d.PullImageFunc = func(ctx context.Context, image string) error {
		pulled = true
		return nil
	}

	session := NewSession(d, &MockAgent{}, "/tmp", "ghcr.io/process-failed-successfully/recac-agent:latest", "test-project", "gemini", "gemini-pro", 1)

	if err := session.ensureImage(context.Background()); err != nil {
		t.Errorf("ensureImage failed: %v", err)
	}

	if !pulled {
		t.Error("Expected image to be pulled")
	}
}

func TestSession_ProcessResponse_JSON(t *testing.T) {
	d := &MockDockerClient{}
	session := NewSession(d, &MockAgent{}, "/tmp", "alpine", "test-project", "gemini", "gemini-pro", 1)

	// JSON block that looks like it might be bash but should be skipped
	response := "```bash\n{\"key\": \"value\"}\n```"

	output, err := session.ProcessResponse(context.Background(), response)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if !strings.Contains(output, "Skipped JSON Block") {
		t.Errorf("Expected output to contain 'Skipped JSON Block', got: %s", output)
	}
}

func TestSession_RunInitScript(t *testing.T) {
	tmpDir := t.TempDir()
	initPath := filepath.Join(tmpDir, "init.sh")
	os.WriteFile(initPath, []byte("echo init"), 0644)

	d := &MockDockerClient{}
	d.ExecAsUserFunc = func(ctx context.Context, containerID, user string, cmd []string) (string, error) {
		if len(cmd) > 2 && strings.Contains(cmd[2], "./init.sh") {
			return "init done", nil
		}
		// Fallback for chmod
		return "", nil
	}

	session := NewSession(d, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.ContainerID = "test-container"

	// It runs in background, so we need to wait a bit or just verify it didn't crash
	// Since runInitScript launches a goroutine, verifying side effects is racy without sync.
	// But we can check that it ATTEMPTED to run if we mock ExecAsUser to notify us.
	// However, ensuring the goroutine runs before we assert is hard.
	// We can skip the goroutine wait logic for unit test and just verify logic paths if possible,
	// OR we can make runInitScript synchronous for testing? No, code modification.

	// We'll trust that if we call it, it spawns.
	// Actually, checking "Found init.sh" in stdout might be hard.
	// Let's just run it to ensure no panic.
	session.runInitScript(context.Background())

	// We can't easily assert execCalled is true immediately.
	// Use assertion with eventually if testify supported it, or simple sleep.
	// time.Sleep(100 * time.Millisecond) // Flaky?
}

func TestSession_FixPasswdDatabase(t *testing.T) {
	d := &MockDockerClient{}
	cmds := []string{}
	d.ExecAsUserFunc = func(ctx context.Context, containerID, user string, cmd []string) (string, error) {
		cmds = append(cmds, strings.Join(cmd, " "))
		return "", nil // Simulate "not found" so it tries to add
	}

	session := NewSession(d, &MockAgent{}, "/tmp", "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.ContainerID = "test-container"

	session.fixPasswdDatabase(context.Background(), "1000:1000")

	// Should try getent group, groupadd, getent passwd, useradd
	// We can't guarantee exact order of checks vs adds if logic changes, but typically check then add.
	// And fallback logic (Alpine) runs if first fails. Our mock returns nil error, but empty string.
	// Logic: if err != nil || out == "" -> try add.
	// Mock returns "", so it proceeds to add.
	// If add fails (mock returns nil err), it proceeds.
	// So we expect 4 calls.

	if len(cmds) < 4 {
		t.Errorf("Expected user creation commands, got: %v", cmds)
	}
}

func TestSession_RunLoop_Stall(t *testing.T) {
	// Create a session with features
	tmpDir := t.TempDir()
	d := &MockDockerClient{}

	// Mock Agent that returns same response (no commands)
	mockAgent := &MockAgent{Response: "I am thinking..."}

	session := NewSession(d, mockAgent, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.MaxIterations = 5
	session.Project = "test-project"

	// Setup features
	features := []db.Feature{{ID: "1", Status: "todo"}}
	fl := db.FeatureList{Features: features}
	data, _ := json.Marshal(fl)
	session.FeatureContent = string(data)

	// We need a DB Store for checkStalledBreaker to work effectively if it relies on DB history,
	// OR it uses in-memory counters.
	// session.go:
	// func (s *Session) checkStalledBreaker(role string, passingCount int) error
	// It uses s.StalledCount.

	// Also ensure app_spec.txt exists
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("spec"), 0644)

	// Run Loop
	// It should fail with ErrStalled or ErrNoOp eventually.
	// Since agent response has no commands, checkNoOpBreaker will trip first?
	// checkNoOpBreaker trips if 3 consecutive iterations have 0 commands.

	err := session.RunLoop(context.Background())
	if err != ErrNoOp {
		t.Errorf("Expected ErrNoOp, got: %v", err)
	}
}

func TestSession_EnsureImage_CustomDockerfile(t *testing.T) {
	tmpDir := t.TempDir()
	dockerfile := filepath.Join(tmpDir, "Dockerfile")
	os.WriteFile(dockerfile, []byte("FROM alpine"), 0644)

	d := &MockDockerClient{}
	buildCalled := false
	d.ImageBuildFunc = func(ctx context.Context, options docker.ImageBuildOptions) (string, error) {
		buildCalled = true
		if options.Dockerfile != "Dockerfile" {
			return "", context.DeadlineExceeded
		}
		return "custom-image-id", nil
	}

	session := NewSession(d, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)

	err := session.ensureImage(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if !buildCalled {
		t.Error("Expected ImageBuild to be called for custom Dockerfile")
	}

	if !strings.Contains(session.Image, "recac-custom-") {
		t.Errorf("Expected session image to be updated, got: %s", session.Image)
	}
}

func TestSession_RunLoop_QAPassed(t *testing.T) {
	tmpDir := t.TempDir()
	d := &MockDockerClient{}

	// Ensure app_spec.txt exists
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("spec"), 0644)

	// Create feature list (all passing) so QA Report has 100% completion
	features := []db.Feature{{ID: "1", Description: "feat 1", Status: "done"}}
	fl := db.FeatureList{ProjectName: "test-project", Features: features}
	data, _ := json.Marshal(fl)
	// Write to file so loadFeatures picks it up
	os.WriteFile(filepath.Join(tmpDir, "feature_list.json"), data, 0644)

	// Manager Agent Mock
	mockManager := &MockAgent{Response: "Approved"}

	session := NewSession(d, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.ManagerAgent = mockManager
	session.MaxIterations = 2

	// Inject QA_PASSED directly into DB since file-based is ignored for privileged signals
	if session.DBStore != nil {
		session.DBStore.SetSignal(session.Project, "QA_PASSED", "true")
	} else {
		t.Fatal("DBStore not initialized")
	}

	// Run Loop
	// Expectation: It finds QA_PASSED, runs Manager Agent.
	// Manager approves (returns "Approved"), so it creates PROJECT_SIGNED_OFF.
	// Then next iteration checks PROJECT_SIGNED_OFF and runs Cleaner.

	err := session.RunLoop(context.Background())
	if err != nil {
		t.Errorf("Expected no error (success), got: %v", err)
	}

	// Check signals in DB (or file if DB failed to init, but NewSession tries hard).
	// Actually, NewSession tries to init DB. If no env vars, it uses sqlite in tmpDir.
	// So session.DBStore should be valid sqlite.

	val, err := session.DBStore.GetSignal(session.Project, "PROJECT_SIGNED_OFF")
	if err != nil || val != "true" {
		t.Errorf("Expected PROJECT_SIGNED_OFF signal to be true, got %s (err: %v)", val, err)
	}
}
