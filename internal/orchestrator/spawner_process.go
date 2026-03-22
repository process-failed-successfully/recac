package orchestrator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"recac/internal/runner"
)

// ProcessSpawner implements the Spawner interface by running the agent as a local child process.
type ProcessSpawner struct {
	Logger            *slog.Logger
	AgentProvider     string
	AgentModel        string
	SessionManager    ISessionManager
	MaxIterations     int
	ManagerFrequency  int
	TaskMaxIterations int

	activeCmds map[string]*exec.Cmd
	logFiles   map[string]string // Maps job ID to log file path
	mu         sync.Mutex
}

// NewProcessSpawner creates a new ProcessSpawner.
func NewProcessSpawner(
	logger *slog.Logger,
	agentProvider string,
	agentModel string,
	sessionManager ISessionManager,
	maxIterations int,
	managerFrequency int,
	taskMaxIterations int,
) *ProcessSpawner {
	return &ProcessSpawner{
		Logger:            logger,
		AgentProvider:     agentProvider,
		AgentModel:        agentModel,
		SessionManager:    sessionManager,
		MaxIterations:     maxIterations,
		ManagerFrequency:  managerFrequency,
		TaskMaxIterations: taskMaxIterations,
		activeCmds:        make(map[string]*exec.Cmd),
		logFiles:          make(map[string]string),
	}
}

// Spawn starts the agent process for the given WorkItem.
func (s *ProcessSpawner) Spawn(ctx context.Context, item WorkItem) error {
	// 1. Create Temporary Workspace
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("recac-agent-%s-*", item.ID))
	if err != nil {
		return fmt.Errorf("failed to create temporary workspace: %w", err)
	}

	// Ensure cleanup of map entries on exit
	defer func() {
		s.mu.Lock()
		delete(s.activeCmds, item.ID)
		s.mu.Unlock()
	}()

	s.Logger.Info("Creating local workspace for job", "id", item.ID, "dir", tempDir)

	// 2. Setup Log File
	logPath := filepath.Join(tempDir, "logs.txt")
	logFile, err := os.Create(logPath)
	if err != nil {
		os.RemoveAll(tempDir)
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	s.mu.Lock()
	s.logFiles[item.ID] = logPath
	s.mu.Unlock()

	// 3. Prepare Environment
	envMap := collectAgentEnvVars(item, s.AgentProvider, s.AgentModel)
	envMap["RECAC_HOST_WORKSPACE_PATH"] = tempDir
	var env []string
	// Inherit current environment but override with envMap
	currentEnv := os.Environ()
	for _, e := range currentEnv {
		env = append(env, e)
	}
	for k, v := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(env)

	// 4. Construct Command
	agentCmdArgs := []string{
		"--jira", item.ID,
		"--project", item.ID,
		"--path", tempDir,
		"--detached=false",
		"--cleanup=false",
		"--verbose",
		"--allow-dirty",
		"--repo-url", item.RepoURL,
		"--max-iterations", fmt.Sprintf("%d", s.MaxIterations),
		"--manager-frequency", fmt.Sprintf("%d", s.ManagerFrequency),
		"--task-max-iterations", fmt.Sprintf("%d", s.TaskMaxIterations),
	}

	cmdArgs := ConstructShellCommand(append([]string{"recac-agent"}, agentCmdArgs...))
	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	cmd.Env = env
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// 5. Create Session State
	session := &runner.SessionState{
		Name:           item.ID,
		StartTime:      time.Now(),
		Command:        cmdArgs,
		Workspace:      tempDir,
		Status:         "running",
		Type:           "orchestrated-process",
		AgentStateFile: filepath.Join(tempDir, ".agent_state.json"),
		StartCommitSHA: "",
	}

	if err := s.SessionManager.SaveSession(session); err != nil {
		s.Logger.Error("failed to save session, cleaning up workspace", "error", err)
		os.RemoveAll(tempDir)
		return fmt.Errorf("failed to save session state: %w", err)
	}

	// 6. Start the Process
	s.Logger.Info("Starting local agent process", "work_item", item.ID)
	if err := cmd.Start(); err != nil {
		s.Logger.Error("failed to start agent process", "error", err)
		session.Status = "failed"
		session.EndTime = time.Now()
		_ = s.SessionManager.SaveSession(session)
		os.RemoveAll(tempDir)
		s.mu.Lock()
		delete(s.logFiles, item.ID)
		s.mu.Unlock()
		return fmt.Errorf("failed to start agent process: %w", err)
	}

	s.mu.Lock()
	s.activeCmds[item.ID] = cmd
	s.mu.Unlock()

	// 7. Wait for Process
	waitErr := cmd.Wait()

	// 8. Update Session
	session.EndTime = time.Now()
	if waitErr != nil {
		s.Logger.Error("Agent process failed", "work_item", item.ID, "error", waitErr)
		session.Status = "failed"
	} else {
		s.Logger.Info("Agent process completed successfully", "work_item", item.ID)
		session.Status = "completed"
	}

	if err := s.SessionManager.SaveSession(session); err != nil {
		s.Logger.Error("failed to update session state after completion", "work_item", item.ID, "error", err)
	}

	if waitErr != nil {
		return fmt.Errorf("agent process failed: %w", waitErr)
	}

	return nil
}

// Cleanup removes the workspace created by the Spawner.
func (s *ProcessSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	s.Logger.Info("Cleaning up local workspace", "work_item", item.ID)

	s.mu.Lock()
	logPath, ok := s.logFiles[item.ID]
	if ok {
		delete(s.logFiles, item.ID)
	}
	s.mu.Unlock()

	if ok {
		// Log path is inside the temp dir, so we can infer the temp dir
		tempDir := filepath.Dir(logPath)
		if err := os.RemoveAll(tempDir); err != nil {
			return fmt.Errorf("failed to remove temporary directory %s: %w", tempDir, err)
		}
	}

	return nil
}

// Cancel terminates a running process by sending SIGTERM.
func (s *ProcessSpawner) Cancel(ctx context.Context, jobID string) error {
	s.Logger.Info("Canceling job process", "job_id", jobID)

	s.mu.Lock()
	cmd, ok := s.activeCmds[jobID]
	s.mu.Unlock()

	if !ok || cmd == nil || cmd.Process == nil {
		return fmt.Errorf("job %s is not actively running as a process", jobID)
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		s.Logger.Error("failed to send SIGTERM to process", "job_id", jobID, "error", err)
		return fmt.Errorf("failed to terminate process: %w", err)
	}

	return nil
}

// GetLogs returns a ReadCloser for the process logs.
func (s *ProcessSpawner) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) {
	s.mu.Lock()
	logPath, ok := s.logFiles[jobID]
	s.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("no logs found for job %s", jobID)
	}

	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", logPath, err)
	}

	return file, nil
}

// Ping verifies if the recac-agent binary exists in PATH.
func (s *ProcessSpawner) Ping(ctx context.Context) error {
	if _, err := exec.LookPath("recac-agent"); err != nil {
		return fmt.Errorf("recac-agent not found in PATH: %w", err)
	}
	return nil
}
