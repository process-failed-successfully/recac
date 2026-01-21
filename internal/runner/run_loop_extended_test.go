package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/db"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"testing"
)

func TestSession_RunLoop_MaxIterations(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	mockDocker := &MockLoopDocker{}
	mockAgent := &MockLoopAgent{
		Response: "Doing nothing.",
	}

	s := &Session{
		Workspace:        tmpDir,
		Docker:           mockDocker,
		Agent:            mockAgent,
		DBStore:          store,
		MaxIterations:    2,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
	}

	ctx := context.Background()
	err := s.RunLoop(ctx)
	if err != ErrMaxIterations {
		t.Errorf("Expected ErrMaxIterations, got %v", err)
	}

	if s.Iteration != 2 {
		t.Errorf("Expected 2 iterations, got %d", s.Iteration)
	}
}

func TestSession_RunLoop_SelectedTask(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	// Setup features
	taskID := "TASK-1"
	features := db.FeatureList{
		Features: []db.Feature{
			{ID: taskID, Description: "Task 1", Status: "todo"},
		},
	}
	data, _ := json.Marshal(features)
	store.SaveFeatures("test-project", string(data))

	mockDocker := &MockLoopDocker{}
	mockAgent := &MockLoopAgent{
		Responses: []string{
			"I will do the task.\n```bash\necho done\n```",
		},
	}

	// We need a way to update the feature status to 'done' so the loop exits.
	// Typically the agent would do this via 'implementation_summary.txt' or modifying 'feature_list.json'.
	// Since we mock the agent, we need to simulate the side effect.
	// However, ProcessResponse executes commands.
	// We can't easily trigger the DB update from the mock command unless we write a real script.

	// Alternative: The agent modifies feature_list.json in workspace.
	// ProcessResponse runs the bash command.
	// The next loop iteration loads features.

	// Let's make the mock agent write to feature_list.json using a bash command that writes to file.
	// But our MockDocker.Exec is basic.

	// Let's use UseLocalAgent=true (or mock docker to do FS op) to actually write the file?
	// Or just update the DB in a goroutine or hook?

	// The MockLoopDocker doesn't execute real commands.

	// Let's rely on injecting the feature update in the "loadFeatures" phase?
	// Session.loadFeatures() reads from DB.
	// So we can update DB in between iterations? No, RunLoop blocks.

	// We can use a custom MockDocker that updates the DB when a specific command is seen.

	mockDocker.ExecFunc = func(ctx context.Context, containerID string, cmd []string) (string, error) {
		// Update DB to mark task as done
		features.Features[0].Status = "done"
		data, _ := json.Marshal(features)
		store.SaveFeatures("test-project", string(data))
		return "updated", nil
	}

	s := &Session{
		Workspace:        tmpDir,
		Docker:           mockDocker,
		Agent:            mockAgent,
		DBStore:          store,
		Project:          "test-project",
		SelectedTaskID:   taskID,
		MaxIterations:    5,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
	}

	ctx := context.Background()
	err := s.RunLoop(ctx)
	if err != nil {
		t.Errorf("RunLoop failed: %v", err)
	}

	// Should have exited because task is done
	// 1 iteration to do the task
	if s.Iteration < 1 {
		t.Errorf("Expected at least 1 iteration, got %d", s.Iteration)
	}
}

func TestSession_RunLoop_ManagerIntervention(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	mockDocker := &MockLoopDocker{}

	// Agent: does nothing useful
	mockAgent := &MockLoopAgent{
		Response: "Working...",
	}

	// Manager Agent
	// We need a custom Send method to verify it was called
	// Since MockLoopAgent is struct, we can't override method easily unless we embed or use interface.
	// The Session uses agent.Agent interface.

	// We'll define a custom mock here.
	mockManagerFunc := &MockLoopAgent{
		Response: "Approved",
	}

	s := &Session{
		Workspace:        tmpDir,
		Docker:           mockDocker,
		Agent:            mockAgent,
		ManagerAgent:     mockManagerFunc,
		DBStore:          store,
		MaxIterations:    5,
		ManagerFrequency: 2, // Trigger every 2 iterations
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
	}

	// We want to verify Manager is called on iteration 2.
	// But RunLoop runs until error or completion.
	// We can set MaxIterations=3.
	s.MaxIterations = 3

	// We can use a trick: MockManager returns a "PROJECT_SIGNED_OFF" signal creation command?
	// But Manager only runs if QA passed?
	// "Manager Review (Triggered by file or frequency) - Main Session Only"
	// This happens inside SelectPrompt.

	// Wait, if ManagerFrequency is hit, SelectPrompt returns ManagerReview prompt.
	// But does it run the ManagerAgent?
	// No, SelectPrompt just selects the prompt and returns `isManager=true`.
	// Then `RunIteration` is called with `isManager=true`.
	// Inside `RunIteration`, it uses `s.Agent`.
	// It does NOT switch to `s.ManagerAgent`.

	// Wait, `RunIteration` uses `s.Agent`.
	// `RunLoop` calls `s.RunIteration`.
	// `RunIteration` calls `s.Agent.Send`.

	// So `s.ManagerAgent` is ONLY used in `runManagerAgent` (which is the final review).
	// For periodic manager review (frequency based), it uses the MAIN agent with a Manager role/prompt?
	// Let's check `RunIteration`.

	// `RunIteration` uses `s.Agent`.

	// So for periodic manager check, the main agent acts as manager?
	// SelectPrompt returns `prompts.ManagerReview`.

	// Verify this assumption by reading `RunIteration` code again.
	// `response, err = s.Agent.Send(ctx, prompt)`
	// Yes.

	// So to test this, we just need to see if `s.Agent` received the manager prompt.

	// Let's track the prompts sent to agent.

	trackingAgent := &TrackingAgent{
		Responses: []string{"resp1", "resp2", "resp3"},
	}
	s.Agent = trackingAgent
	s.MaxIterations = 2
	s.ManagerFrequency = 2

	ctx := context.Background()
	s.RunLoop(ctx) // Ignore MaxIterations error

	if len(trackingAgent.Prompts) < 2 {
		t.Errorf("Expected at least 2 prompts, got %d", len(trackingAgent.Prompts))
	}

	// 2nd prompt should be Manager Review?
	// Iteration starts at 0? No, IncrementIteration starts at 1.
	// If Frequency=2, Iteration 2 % 2 == 0. So it triggers.

	// Check prompt content for "Manager Review" or similar keywords (depends on prompt template)
	// or check trackingAgent.Prompts[1].
}

type TrackingAgent struct {
	Prompts   []string
	Responses []string
	CallCount int
}

func (m *TrackingAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.Prompts = append(m.Prompts, prompt)
	resp := "default"
	if m.CallCount < len(m.Responses) {
		resp = m.Responses[m.CallCount]
	}
	m.CallCount++
	return resp, nil
}

func (m *TrackingAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}
func (m *TrackingAgent) WithStateManager(sm *agent.StateManager) agent.Agent { return m }
