package runner

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/agent/prompts"
	"recac/internal/db"
	"recac/internal/docker"
	"recac/internal/git"
	"recac/internal/security"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"recac/internal/notify"
	"recac/internal/telemetry"

	"github.com/spf13/viper"
)

var ErrBlocker = errors.New("blocker detected")
var ErrMaxIterations = errors.New("maximum iterations reached")
var ErrNoOp = errors.New("circuit breaker: no-op loop")
var ErrStalled = errors.New("circuit breaker: stalled progress")

var bashBlockRegex = regexp.MustCompile("(?s)```bash\\s*(.*?)\\s*```")

type Session struct {
	Docker           DockerClient
	Agent            agent.Agent
	Workspace        string
	Image            string
	SpecFile         string
	Iteration        int
	MaxIterations    int
	ManagerFrequency int
	ManagerFirst     bool
	StreamOutput     bool
	Model            string
	AgentStateFile   string              // Path to agent state file (.agent_state.json)
	StateManager     *agent.StateManager // State manager for agent state persistence
	DBStore          db.Store            // Persistent database store
	Scanner          security.Scanner    // Security scanner
	ContainerID      string              // Container ID for cleanup

	// Dependency Injection for Testing (optional)
	// Agent Clients
	CodingAgent   agent.Agent
	CleanerAgent  agent.Agent
	QAAgent       agent.Agent
	ManagerAgent  agent.Agent
	AgentProvider string // Specific provider for this session
	AgentModel    string // Specific model for this session

	// Circuit Breaker State
	LastFeatureCount int // Number of passing features last time we checked
	StalledCount     int // Number of iterations without feature progress
	NoOpCount        int // Number of iterations without executed commands

	// Multi-Agent support
	SelectedTaskID            string // If set, the agent should focus ONLY on this task
	MaxAgents                 int    // Maximum number of parallel agents
	OwnsDB                    bool   // Whether this session owns the DB connection (and should close it)
	Project                   string // Project identifier for telemetry
	TaskMaxIterations         int    // Max iterations for sub-tasks (if applicable)
	Notifier                  notify.Notifier
	BaseBranch                string // Base Branch for merge guardrails
	SkipQA                    bool   // Skip QA phase and auto-complete
	AutoMerge                 bool   // Automatically merge PRs
	JiraClient                JiraClient
	JiraTicketID              string
	RepoURL                   string       // Repository URL for links
	SlackThreadTS             string       // Thread Timestamp for Slack conversations
	SuppressStartNotification bool         // Suppress "Session Started" notification (for sub-tasks)
	UseLocalAgent             bool         // Execute commands locally (e.g. inside K8s pod) instead of spawning Docker container
	SpecContent               string       // Explicit specification content (e.g. from Jira)
	FeatureContent            string       // Explicit feature list JSON content (authoritative)
	Logger                    *slog.Logger // Structured logger for this session

	mu sync.RWMutex // Protects concurrent access to Iteration, SlackThreadTS, ContainerID
}

// JiraClient defines the interface for Jira operations needed by the session
type JiraClient interface {
	AddComment(ctx context.Context, ticketID, comment string) error
	SmartTransition(ctx context.Context, ticketID, targetNameOrID string) error
}

// NewSession creates a new worker session
func NewSession(d DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *Session {
	// Default to "unknown" if project is empty
	if project == "" {
		project = "unknown"
	}

	// Default agent state file path in workspace
	stateFile := ".agent_state.json"
	agentStateFile := filepath.Join(workspace, stateFile)
	stateManager := agent.NewStateManager(agentStateFile)

	// Initialize DB Store
	dbType := os.Getenv("RECAC_DB_TYPE")
	dbURL := os.Getenv("RECAC_DB_URL")

	if dbType == "" {
		dbType = "sqlite"
		if dbURL == "" {
			dbURL = filepath.Join(workspace, ".recac.db")
		}
	} else if dbType == "sqlite" && dbURL == "" {
		dbURL = filepath.Join(workspace, ".recac.db")
	}

	// Initialize DB Store with Retry Logic
	var dbStore db.Store
	storeConfig := db.StoreConfig{
		Type:             dbType,
		ConnectionString: dbURL,
	}

	// Retry loop for DB connection (up to 30 seconds)
	var err error
	maxRetries := 6
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			fmt.Fprintf(os.Stderr, "[Session] Retrying DB connection (%d/%d)...\n", i+1, maxRetries)
			time.Sleep(5 * time.Second)
		}
		dbStore, err = db.NewStore(storeConfig)
		if err == nil {
			break
		}
		fmt.Fprintf(os.Stderr, "[Session] Failed to initialize DB store (%s): %v\n", dbType, err)
	}

	if err != nil {
		// Critical failure - Fail Fast
		fmt.Fprintf(os.Stderr, "[Session] CRITICAL: Could not connect to database after retries. Exiting.\n")
		os.Exit(1)
	} else {
		// Success
		fmt.Fprintf(os.Stderr, "[Session] DB Store initialized successfully: type=%s, project=%s\n", dbType, project)
		slog.Info("[DB] Store initialized successfully", "type", dbType, "project", project)
	}

	// Initialize Security Scanner
	scanner := security.NewRegexScanner()

	// Create agents/logs directory in the current working directory (host)
	// This is where Promtail expects to find them based on docker-compose.monitoring.yml
	cwd, _ := os.Getwd()
	agentsLogsDir := filepath.Join(cwd, "agents", "logs")
	if err := os.MkdirAll(agentsLogsDir, 0755); err != nil {
		fmt.Printf("Warning: Failed to create agents/logs directory: %v\n", err)
	} else {
		// Initialize session log file
		timestamp := time.Now().Format("20060102-150405")
		logFileName := fmt.Sprintf("%s_agent_%s_%s.log", project, project, timestamp)
		logFilePath := filepath.Join(agentsLogsDir, logFileName)

		// Re-initialize telemetry logger with the session log file
		// Note: We use the global 'verbose' setting
		// We still init global logger for backward compatibility and simpler calls where session isn't available
		telemetry.InitLogger(viper.GetBool("verbose"), logFilePath, false)
		fmt.Printf("Session logs will be written to: %s\n", logFilePath)
	}

	// Create session logger
	// We want to persist it in the session so it can be customized (e.g. with attributes)
	// For now, we reuse the configuration logic but ideally we'd pass this logger instance around.
	// Since we called InitLogger above, slog.Default() is set.
	// But let's create an explicit one too.
	logger := telemetry.NewLogger(viper.GetBool("verbose"), "", false)
	if project != "" {
		logger = logger.With("project", project)
	}

	return &Session{
		Docker:           d,
		Agent:            a,
		Workspace:        workspace,
		Image:            image,
		Project:          project,
		AgentProvider:    provider,
		AgentModel:       model,
		SpecFile:         "app_spec.txt",
		MaxIterations:    20, // Default
		ManagerFrequency: 5,  // Default
		AgentStateFile:   agentStateFile,
		StateManager:     stateManager,
		DBStore:          dbStore,
		OwnsDB:           true,
		Scanner:          scanner,
		MaxAgents:        maxAgents,
		Notifier:         notify.NewManager(telemetry.LogInfof),
		UseLocalAgent:    os.Getenv("KUBERNETES_SERVICE_HOST") != "",
		Logger:           logger,
	}
}

// NewSessionWithStateFile creates a session with a specific agent state file (for restoring sessions)
func NewSessionWithStateFile(d DockerClient, a agent.Agent, workspace, image, project, agentStateFile, provider, model string, maxAgents int) *Session {
	if project == "" {
		project = "unknown"
	}
	stateManager := agent.NewStateManager(agentStateFile)

	// Initialize DB Store
	dbType := os.Getenv("RECAC_DB_TYPE")
	dbURL := os.Getenv("RECAC_DB_URL")

	if dbType == "" {
		dbType = "sqlite"
		if dbURL == "" {
			dbURL = filepath.Join(workspace, ".recac.db")
		}
	} else if dbType == "sqlite" && dbURL == "" {
		dbURL = filepath.Join(workspace, ".recac.db")
	}

	var dbStore db.Store
	storeConfig := db.StoreConfig{
		Type:             dbType,
		ConnectionString: dbURL,
	}

	if s, err := db.NewStore(storeConfig); err != nil {
		fmt.Printf("Warning: Failed to initialize DB store (%s): %v\n", dbType, err)
	} else {
		dbStore = s
	}

	// Initialize Security Scanner
	scanner := security.NewRegexScanner()

	// Create agents/logs directory in the current working directory (host)
	// This is where Promtail expects to find them based on docker-compose.monitoring.yml
	cwd, _ := os.Getwd()
	agentsLogsDir := filepath.Join(cwd, "agents", "logs")
	if err := os.MkdirAll(agentsLogsDir, 0755); err != nil {
		fmt.Printf("Warning: Failed to create agents/logs directory: %v\n", err)
	} else {
		// Initialize session log file
		timestamp := time.Now().Format("20060102-150405")
		logFileName := fmt.Sprintf("%s_agent_%s_%s.log", project, project, timestamp)
		logFilePath := filepath.Join(agentsLogsDir, logFileName)

		// Re-initialize telemetry logger with the session log file
		// Note: We use the global 'verbose' setting (viper)
		telemetry.InitLogger(viper.GetBool("verbose"), logFilePath, false)
		fmt.Printf("Session logs will be written to: %s\n", logFilePath)
	}

	logger := telemetry.NewLogger(viper.GetBool("verbose"), "", false)
	if project != "" {
		logger = logger.With("project", project)
	}

	return &Session{
		Docker:           d,
		Agent:            a,
		Workspace:        workspace,
		Image:            image,
		Project:          project,
		AgentProvider:    provider,
		AgentModel:       model,
		SpecFile:         "app_spec.txt",
		MaxIterations:    20, // Default
		ManagerFrequency: 5,  // Default
		AgentStateFile:   agentStateFile,
		StateManager:     stateManager,
		DBStore:          dbStore,
		OwnsDB:           true,
		Scanner:          scanner,
		MaxAgents:        maxAgents,
		Notifier:         notify.NewManager(telemetry.LogInfof),
		Logger:           logger,
	}
}

// NewSessionWithConfig creates a session with specific provider/model settings.
// This is used for sub-agents or when overriding global config.
func NewSessionWithConfig(workspace, project, provider, model string, dbStore db.Store) *Session {
	// Default to "unknown" if project is empty
	if project == "" {
		project = "unknown"
	}

	// Default agent state file path in workspace
	stateFile := ".agent_state.json"
	agentStateFile := filepath.Join(workspace, stateFile)
	stateManager := agent.NewStateManager(agentStateFile)

	// Initialize Security Scanner
	scanner := security.NewRegexScanner()

	// Create agents/logs directory in the current working directory (host)
	cwd, _ := os.Getwd()
	agentsLogsDir := filepath.Join(cwd, "agents", "logs")
	if err := os.MkdirAll(agentsLogsDir, 0755); err != nil {
		fmt.Printf("Warning: Failed to create agents/logs directory: %v\n", err)
	} else {
		// Initialize session log file
		timestamp := time.Now().Format("20060102-150405")
		logFileName := fmt.Sprintf("%s_agent_%s_%s.log", project, project, timestamp)
		logFilePath := filepath.Join(agentsLogsDir, logFileName)

		// Re-initialize telemetry logger with the session log file
		telemetry.InitLogger(viper.GetBool("verbose"), logFilePath, false)
		fmt.Printf("Session logs will be written to: %s\n", logFilePath)
	}

	logger := telemetry.NewLogger(viper.GetBool("verbose"), "", false)
	if project != "" {
		logger = logger.With("project", project)
	}

	return &Session{
		Workspace:        workspace,
		Project:          project,
		AgentProvider:    provider,
		AgentModel:       model,
		DBStore:          dbStore,
		SpecFile:         "app_spec.txt",
		MaxIterations:    20, // Default
		ManagerFrequency: 5,  // Default
		AgentStateFile:   agentStateFile,
		StateManager:     stateManager,
		OwnsDB:           false, // This session does not own the DB, it's passed in
		Scanner:          scanner,
		Notifier:         notify.NewManager(telemetry.LogInfof),
		Logger:           logger,
	}
}

// LoadAgentState loads agent state from disk if it exists
// LoadAgentState loads agent state from disk if it exists
func (s *Session) LoadAgentState() error {
	if s.StateManager == nil {
		return nil // No state manager configured
	}

	// GUARDRAIL: Automatically ensure state files are ignored by git
	if err := EnsureStateIgnored(s.Workspace); err != nil {
		fmt.Printf("Warning: Failed to ensure state files are ignored: %v\n", err)
	}

	// INVALID STATE GUARDRAIL: Load with safeguard to auto-delete corrupt state
	var state agent.State
	if err := LoadSafeguardedState(s.AgentStateFile, &state); err != nil {
		return fmt.Errorf("failed to load safeguarded state: %w", err)
	}

	// Manually inject the loaded state into the StateManager to ensure it's in sync
	// We need to extend StateManager or just accept that the next Save() will overwrite it.
	// However, StateManager.Load() is just a reader. To "inject" it, we rely on the fact
	// that we just loaded it. But wait, we need to return it?
	// The original code was: state, err := s.StateManager.Load()

	// Since StateManager doesn't preserve state in memory (it loads on demand),
	// we assume that if LoadSafeguardedState succeeded, the file is valid.
	// So we can just call s.StateManager.Load() safely now.
	// OR we can trust LoadSafeguardedState if it wrote it back? No, it only reads/deletes.

	// If LoadSafeguardedState deleted the file, StateManager.Load() will return empty state (handled internally).

	loadedState, err := s.StateManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load agent state: %w", err)
	}

	// If state has memory/history, we can use it to restore context
	if len(loadedState.Memory) > 0 {
		fmt.Printf("Loaded agent state: %d memory items, %d history messages\n", len(loadedState.Memory), len(loadedState.History))
	}

	// Log token usage if available
	if loadedState.TokenUsage.TotalTokens > 0 {
		fmt.Printf("Token usage: total=%d (prompt=%d, response=%d), current=%d/%d, truncations=%d\n",
			loadedState.TokenUsage.TotalTokens,
			loadedState.TokenUsage.TotalPromptTokens,
			loadedState.TokenUsage.TotalResponseTokens,
			loadedState.CurrentTokens,
			loadedState.MaxTokens,
			loadedState.TokenUsage.TruncationCount)
	}

	return nil
}

// InitializeAgentState initializes agent state with max_tokens from config
func (s *Session) InitializeAgentState(maxTokens int) error {
	if s.StateManager == nil {
		return nil // No state manager configured
	}

	// Also persist the model name
	return s.StateManager.InitializeState(maxTokens, s.AgentModel)
}

// SaveAgentState saves the current agent state to disk
func (s *Session) SaveAgentState() error {
	if s.StateManager == nil {
		return nil // No state manager configured
	}

	// Load current state
	state, err := s.StateManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load state for saving: %w", err)
	}

	// Save state (StateManager will update UpdatedAt timestamp)
	return s.StateManager.Save(state)
}

// ReadSpec reads the application specification file from the workspace.
func (s *Session) ReadSpec() (string, error) {
	path := filepath.Join(s.Workspace, s.SpecFile)

	// 1. Try file first
	content, err := os.ReadFile(path)
	if err == nil {
		// Sync VALID file content to DB so DB stays fresh
		if s.DBStore != nil && len(content) > 0 {
			_ = s.DBStore.SaveSpec(s.Project, string(content))
		}
		return string(content), nil
	}

	// 2. Fallback: SpecContent field (passed from Orchestrator/CLI)
	if s.SpecContent != "" {
		// Initialize file
		_ = os.WriteFile(path, []byte(s.SpecContent), 0644)
		if s.DBStore != nil {
			_ = s.DBStore.SaveSpec(s.Project, s.SpecContent)
		}
		return s.SpecContent, nil
	}

	// 3. Fallback: DB (Authoritative Mirror)
	if s.DBStore != nil {
		dbContent, err := s.DBStore.GetSpec(s.Project)
		if err == nil && dbContent != "" {
			// Restore file from DB
			_ = os.WriteFile(path, []byte(dbContent), 0644)
			return dbContent, nil
		}
	}

	return "", fmt.Errorf("failed to read spec file and no backups found: %w", err)
}

// Start initializes the session environment (Docker container).
func (s *Session) Start(ctx context.Context) error {
	// If a specific task is selected, use a task-specific state file to avoid clobbering
	if s.SelectedTaskID != "" {
		s.AgentStateFile = filepath.Join(s.Workspace, fmt.Sprintf(".agent_state_%s.json", s.SelectedTaskID))
		s.StateManager = agent.NewStateManager(s.AgentStateFile)

		// Inject the new StateManager into the agent if it supports it
		type withSM interface {
			WithStateManager(sm *agent.StateManager) agent.Agent
		}
		// Some agents return the client itself. We try to type assert to common types too.
		if aw, ok := s.Agent.(interface {
			WithStateManager(*agent.StateManager) *agent.GeminiClient
		}); ok {
			aw.WithStateManager(s.StateManager)
		} else if aw, ok := s.Agent.(interface {
			WithStateManager(*agent.StateManager) *agent.OpenAIClient
		}); ok {
			aw.WithStateManager(s.StateManager)
		} else if aw, ok := s.Agent.(interface {
			WithStateManager(*agent.StateManager) *agent.OpenRouterClient
		}); ok {
			aw.WithStateManager(s.StateManager)
		}
	}

	fmt.Printf("Initializing session with image: %s\n", s.Image)

	// Check Docker Daemon
	if s.Docker != nil {
		if err := s.Docker.CheckDaemon(ctx); err != nil {
			fmt.Printf("Warning: Docker check failed: %v. Running in restricted mode.\n", err)
			s.Docker = nil // Disable docker usage if check fails
		}
	} else {
		fmt.Println("Running in restricted mode (no Docker access).")
	}

	// Read Spec
	spec, err := s.ReadSpec()
	if err != nil {
		fmt.Printf("Warning: Failed to read spec: %v\n", err)
	} else {
		fmt.Printf("Loaded spec: %d bytes\n", len(spec))
	}

	// Ensure Image is ready (only if Docker is available)
	if s.Docker != nil {
		if err := s.ensureImage(ctx); err != nil {
			fmt.Printf("Warning: Failed to ensure image %s: %v. Attempting to proceed anyway...\n", s.Image, err)
		}
	}

	// Determine users home directory for config mounting
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Warning: Failed to determine user home dir: %v. Configs will not be mounted.\n", err)
	}

	var extraBinds []string
	if homeDir != "" {
		// Mount configurations if they exist
		// Note: Docker binds require the host path to exist, or it might auto-create as dir (depends on docker version/config).
		// Best practice is to check existence, but for now we follow the Python approach which seemingly just mounts them.
		// However, to avoid creating empty dirs if they don't exist on host, we can check.
		// For now, we'll blindly mount as per requirement to emulate python script behavior effectively.
		extraBinds = append(extraBinds,
			fmt.Sprintf("%s/.gemini:/home/appuser/.gemini", homeDir),
			fmt.Sprintf("%s/.config:/home/appuser/.config", homeDir),
			fmt.Sprintf("%s/.cursor:/home/appuser/.cursor", homeDir),
			fmt.Sprintf("%s/.ssh:/home/appuser/.ssh", homeDir),
		)
	}

	// Determine host user for mapping
	containerUser := ""
	u, _ := user.Current()
	if u != nil {
		containerUser = fmt.Sprintf("%s:%s", u.Uid, u.Gid)
	}

	// 1.5 Mount agent-bridge
	bridgePath, err := s.findAgentBridgeBinary()
	if err != nil {
		fmt.Printf("Warning: Failed to locate agent-bridge binary: %v. Agent CLI tools will not work.\n", err)
	} else {
		// If found in standard location, assume it is present in the container image and skip mount
		// This avoids issues with mounting files over existing files/directories in Docker-in-Docker scenarios
		if bridgePath == "/usr/local/bin/agent-bridge" {
			fmt.Printf("Agent bridge found in standard location %s, skipping mount (assumed present in image)\n", bridgePath)
		} else {
			// Append to extraBinds
			// format: /host/path:/container/path:ro
			extraBinds = append(extraBinds, fmt.Sprintf("%s:/usr/local/bin/agent-bridge:ro", bridgePath))
			fmt.Printf("Mounting agent-bridge from %s to /usr/local/bin/agent-bridge\n", bridgePath)
		}
	}

	// Collect Env Vars to propagate to container
	var env []string
	prefixes := []string{"GIT_", "JIRA_", "RECAC_", "OPENROUTER_", "OPENAI_", "ANTHROPIC_", "GEMINI_"}
	for _, e := range os.Environ() {
		for _, p := range prefixes {
			if strings.HasPrefix(e, p) {
				env = append(env, e)
				break
			}
		}
	}

	// Explicitly inject Session Context (Critical for DB persistence)
	if s.Project != "" {
		env = append(env, fmt.Sprintf("RECAC_PROJECT_ID=%s", s.Project))
	}

	// Run Container (or Skip if Local/Restricted)
	if s.UseLocalAgent || s.Docker == nil {
		if s.Logger != nil {
			s.Logger.Info("Running in Local Agent Mode (K8s detected or restricted). Skipping container spawn.")
		} else {
			fmt.Println("Running in Local Agent Mode (K8s detected or restricted). Skipping container spawn.")
		}
		s.ContainerID = "local"
		s.UseLocalAgent = true
	} else {
		id, err := s.Docker.RunContainer(ctx, s.Image, s.Workspace, extraBinds, env, containerUser)
		if err != nil {
			return err
		}

		s.ContainerID = id
		fmt.Printf("Container started successfully. ID: %s\n", id)

		// Fix Linux passwd database (ensure host UID exists in container)
		if containerUser != "" {
			s.fixPasswdDatabase(ctx, containerUser)
		}
	}

	// Bootstrap Git Config
	if err := s.bootstrapGit(ctx); err != nil {
		fmt.Printf("Warning: Git bootstrapping failed: %v\n", err)
	}

	// Run init.sh if it exists
	s.runInitScript(ctx)

	// Start Notifier (Socket Mode)
	s.Notifier.Start(ctx)

	// Restore Slack Thread TS from DB if available (for session resumption)
	if s.SlackThreadTS == "" && s.DBStore != nil {
		if ts, err := s.DBStore.GetSignal(s.Project, "SLACK_THREAD_TS"); err == nil && ts != "" {
			s.SlackThreadTS = ts
			s.Logger.Info("restored slack thread ts from db", "ts", ts)
		}
	}
	// Notify Start
	if !s.SuppressStartNotification {
		msg := fmt.Sprintf("Project %s: Session Started", s.Project)
		if s.Iteration > 1 {
			msg = fmt.Sprintf("Project %s: Session Resumed (Iteration %d)", s.Project, s.Iteration)
		}

		// Capture timestamp for threading
		ts, _ := s.Notifier.Notify(ctx, notify.EventStart, msg, s.SlackThreadTS)
		if s.SlackThreadTS == "" {
			s.SlackThreadTS = ts
			// Persist new thread TS
			if s.DBStore != nil && ts != "" {
				if err := s.DBStore.SetSignal(s.Project, "SLACK_THREAD_TS", ts); err != nil {
					fmt.Printf("Warning: Failed to persist Slack Thread TS: %v\n", err)
				}
			}
		}
	}

	return nil
}

// ensureImage ensures the agent image exists locally, pulling or building if needed.
func (s *Session) ensureImage(ctx context.Context) error {
	if s.Docker == nil {
		fmt.Println("Docker not available available. Skipping image check (assuming local execution or pre-pulled).")
		return nil
	}

	// 1. Check for custom Dockerfile in workspace
	// 1. Check if workspace has a Dockerfile. If so, building is mandatory to allow customization.
	workspaceDockerfile := filepath.Join(s.Workspace, "Dockerfile")
	if _, err := os.Stat(workspaceDockerfile); err == nil {
		fmt.Printf("Custom Dockerfile found at %s. Building image...\n", workspaceDockerfile)
		data, err := os.ReadFile(workspaceDockerfile)
		if err != nil {
			return fmt.Errorf("failed to read workspace Dockerfile: %w", err)
		}

		// Use the workspace path as tag if it's a custom build, or a specific name
		tag := "recac-custom-" + filepath.Base(s.Workspace) + ":latest"

		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		_ = tw.WriteHeader(&tar.Header{Name: "Dockerfile", Size: int64(len(data)), Mode: 0644})
		_, _ = tw.Write(data)
		_ = tw.Close()

		newID, err := s.Docker.ImageBuild(ctx, docker.ImageBuildOptions{
			BuildContext: &buf,
			Tag:          tag,
			Dockerfile:   "Dockerfile",
		})
		if err != nil {
			return fmt.Errorf("failed to build custom image: %w", err)
		}
		fmt.Printf("Custom image built successfully: %s\n", newID)
		s.Image = tag
		return nil
	}

	// 2. If using default GHCR image, ensure it is pulled if missing
	if strings.HasPrefix(s.Image, "ghcr.io/process-failed-successfully/recac-agent") {
		exists, err := s.Docker.ImageExists(ctx, s.Image)
		if err != nil {
			return fmt.Errorf("failed to check image existence: %w", err)
		}

		if !exists {
			fmt.Printf("Default agent image '%s' not found locally. Pulling...\n", s.Image)
			if err := s.Docker.PullImage(ctx, s.Image); err != nil {
				return fmt.Errorf("failed to pull agent image: %w", err)
			}
			fmt.Println("Agent image pulled successfully.")
		}
		return nil
	}

	// 3. Fallback: If using legacy default image name, ensure it's built from our embedded template
	if s.Image == "recac-agent:latest" {
		exists, err := s.Docker.ImageExists(ctx, s.Image)
		if err != nil {
			return fmt.Errorf("failed to check image existence: %w", err)
		}

		if !exists {
			fmt.Println("Legacy agent image 'recac-agent:latest' not found. Building from template...")

			var buf bytes.Buffer
			tw := tar.NewWriter(&buf)
			content := docker.DefaultAgentDockerfile
			_ = tw.WriteHeader(&tar.Header{Name: "Dockerfile", Size: int64(len(content)), Mode: 0644})
			_, _ = tw.Write([]byte(content))
			_ = tw.Close()

			newID, err := s.Docker.ImageBuild(ctx, docker.ImageBuildOptions{
				BuildContext: &buf,
				Tag:          s.Image,
				Dockerfile:   "Dockerfile",
			})
			if err != nil {
				return fmt.Errorf("failed to build legacy agent image: %w", err)
			}
			fmt.Printf("Legacy agent image built successfully: %s\n", newID)
		}
	}

	return nil
}

// Stop cleans up the Docker container.
func (s *Session) Stop(ctx context.Context) error {
	if s.DBStore != nil && s.OwnsDB {
		if err := s.DBStore.Close(); err != nil {
			fmt.Printf("Warning: Failed to close DB store: %v\n", err)
		}
	}

	s.mu.Lock()
	containerID := s.ContainerID
	s.mu.Unlock()

	if containerID == "" || containerID == "local" { // Added "local" check for UseLocalAgent mode
		return nil // No container to clean up or running locally
	}

	fmt.Printf("Stopping container: %s\n", containerID)
	if s.Docker != nil {
		if err := s.Docker.StopContainer(ctx, containerID); err != nil {
			return fmt.Errorf("failed to stop container: %w", err)
		}
	}

	s.mu.Lock()
	s.ContainerID = ""
	s.mu.Unlock()
	fmt.Println("Container stopped successfully")

	return nil
}

// Thread-safe Accessors

func (s *Session) GetIteration() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Iteration
}

func (s *Session) IncrementIteration() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Iteration++
	return s.Iteration
}

func (s *Session) GetSlackThreadTS() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SlackThreadTS
}

func (s *Session) SetSlackThreadTS(ts string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SlackThreadTS = ts
}

func (s *Session) GetContainerID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ContainerID
}

func (s *Session) SetContainerID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ContainerID = id
}


// RunLoop executes the autonomous agent loop.
func (s *Session) RunLoop(ctx context.Context) error {
	// Guard: Ensure Notifier is initialized (mostly for tests using manual struct initialization)
	if s.Notifier == nil {
		s.Notifier = notify.NewManager(func(string, ...interface{}) {})
	}

	s.Logger.Info("entering autonomous run loop")
	// Note: We use the stored SlackThreadTS if available (from startup), otherwise we start a new thread here if needed?
	// But Start() is called before RunLoop(), so s.SlackThreadTS should be set if notifications are enabled.
	// If it's a resume and we don't have the TS persisted, we might start a new thread.
	// For now, let's just log if it's not set.
	if s.GetSlackThreadTS() == "" {
		// Try to send a start message if we missed it (e.g. manual RunLoop call)
		ts, _ := s.Notifier.Notify(ctx, notify.EventStart, fmt.Sprintf("Session Started for Project: %s", s.Project), "")
		s.SetSlackThreadTS(ts)
	} else {
		// Just log context update if needed, but "Session Started" is redundant if checking duplicates.
		// User complained about DUPLICATE messages. If Start() already sent one, RunLoop shouldn't send another top-level one.
		// So we ONLY send if s.SlackThreadTS is empty.
	}

	// Guardrail: Ensure app_spec.txt exists (Source of Truth)
	// We skip this check for Mock mode users who might not have set it up, but for real usage it's mandatory.
	// Actually, user said "Immediately fail if there is no app_spec.txt", so we enforce it strict.
	specPath := filepath.Join(s.Workspace, "app_spec.txt")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		return fmt.Errorf("CRITICAL ERROR: app_spec.txt not found in workspace (%s). This file is required as the source of truth for the project.", s.Workspace)
	}

	// Load agent state if it exists (for session restoration)
	if err := s.LoadAgentState(); err != nil {
		fmt.Printf("Warning: Failed to load agent state: %v\n", err)
		// Continue anyway - state will be created on first save
	}

	// Load DB history if available
	if s.DBStore != nil {
		history, err := s.DBStore.QueryHistory(s.Project, 5)
		if err == nil && len(history) > 0 {
			fmt.Printf("Loaded %d previous observations from DB history.\n", len(history))
		}
	}

	// Startup Check: If feature list exists and all passed, mark COMPLETED
	features := s.loadFeatures()
	if len(features) > 0 {
		allPassed := true
		for _, f := range features {
			if !(f.Passes || f.Status == "done" || f.Status == "implemented") {
				allPassed = false
				break
			}
		}
		if allPassed {
			fmt.Println("All features passed! Triggering Project Complete flow.")
			if err := s.createSignal("COMPLETED"); err != nil {
				fmt.Printf("Warning: Failed to create COMPLETED signal: %v\n", err)
			}

			// Final Phase: UI Verification Check
			s.Notifier.Notify(ctx, notify.EventProjectComplete, fmt.Sprintf("Project %s is COMPLETE!", s.Project), s.GetSlackThreadTS())
		}
	}

	// Ensure cleanup on exit (defer cleanup)
	defer func() {
		containerID := s.GetContainerID()
		if containerID != "" {
			fmt.Printf("Cleaning up container: %s\n", containerID)
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			defer cancel()
			if s.Docker != nil {
				if err := s.Docker.StopContainer(cleanupCtx, containerID); err != nil {
					fmt.Printf("Warning: Failed to cleanup container: %v\n", err)
				} else {
					fmt.Println("Container cleaned up successfully")
				}
			}
		}
	}()

	for {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check Max Iterations
		currentIteration := s.GetIteration()
		if s.MaxIterations > 0 && currentIteration >= s.MaxIterations {
			s.Logger.Info("reached max iterations", "max_iterations", s.MaxIterations)
			return ErrMaxIterations
		}

		newIteration := s.IncrementIteration()
		s.Logger.Info("starting iteration", "iteration", newIteration, "task_id", s.SelectedTaskID, "agent_provider", s.AgentProvider, "agent_model", s.AgentModel)
		if s.SelectedTaskID != "" {
			// Log task description snippet for debugging context
			descSnippet := ""
			if len(s.SpecContent) > 50 {
				descSnippet = s.SpecContent[:50] + "..."
			} else {
				descSnippet = s.SpecContent
			}
			s.Logger.Info("assigned task details", "task_id", s.SelectedTaskID, "desc_snippet", descSnippet)
		}

		// Ensure feature list is synced and mirror is up to date
		features = s.loadFeatures()

		// Single-Task Termination: If we are assigned a specific task and it's done, exit.
		if s.SelectedTaskID != "" {
			for _, f := range features {
				if f.ID == s.SelectedTaskID && (f.Passes || f.Status == "done" || f.Status == "implemented") {
					s.Logger.Info("task completed", "task_id", s.SelectedTaskID)
					return nil
				}
			}
		}

		// Handle Lifecycle Role Transitions (Agent-QA-Manager-Cleaner workflow)
		// Prioritize these checks at the beginning of the iteration
		if s.hasSignal("PROJECT_SIGNED_OFF") {
			// MERGE GUARDRAIL: Check for upstream conflicts before accepting sign-off
			if s.BaseBranch != "" {
				s.Logger.Info("checking for upstream changes", "branch", s.BaseBranch)

				// Git Recovery/Retry Loop
				maxRetries := 3
				gitClient := git.NewClient()
				success := false

				for i := 0; i < maxRetries; i++ {
					// 1. Fix Permissions
					if err := s.fixPermissions(ctx); err != nil {
						fmt.Printf("Warning: Failed to fix permissions (attempt %d/%d): %v\n", i+1, maxRetries, err)
					}

					// 2. Fetch
					if err := gitClient.Fetch(s.Workspace, "origin", s.BaseBranch); err == nil {
						// Stash (ignore errors)
						_ = gitClient.Stash(s.Workspace)

						// 3. Attempt Merge
						if err := gitClient.Merge(s.Workspace, "origin/"+s.BaseBranch); err != nil {
							s.Logger.Warn("merge failed", "attempt", i+1, "max", maxRetries, "error", err)

							// ENSURE WE ABORT to clear unmerged files
							_ = gitClient.AbortMerge(s.Workspace)

							// RECOVERY STRATEGIES
							if i < maxRetries-1 {
								s.Logger.Info("attempting git recovery")

								// Recovery Step 1: Remove Locks
								if err := gitClient.Recover(s.Workspace); err != nil {
									s.Logger.Warn("recover failed", "error", err)
								}

								// Recovery Step 2: Clean aggressively
								if err := gitClient.Clean(s.Workspace); err != nil {
									s.Logger.Warn("clean failed", "error", err)
								}

								// Recovery Step 3: Hard Reset to origin/current_feature_branch
								// This is safer than just 'reset --hard' without target
								cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
								cmd.Dir = s.Workspace
								if out, err := cmd.Output(); err == nil {
									currBranch := strings.TrimSpace(string(out))
									_ = gitClient.ResetHard(s.Workspace, "origin", currBranch)
								}
							} else {
								// Final Failure
								s.Logger.Error("critical merge failure", "branch", s.BaseBranch, "attempts", maxRetries)
							}
						} else {
							// Success
							success = true
							if err := gitClient.StashPop(s.Workspace); err != nil {
								s.Logger.Warn("restore stash failed", "error", err)
							}
							s.Logger.Info("branch up-to-date with base")
							break
						}
					} else {
						s.Logger.Warn("fetch failed", "attempt", i+1, "max", maxRetries, "error", err)
						gitClient.Recover(s.Workspace) // Try recovering for next loop
					}
					time.Sleep(2 * time.Second)
				}

				if !success {
					s.Logger.Warn("merge conflict or persistent git error, revoking sign-off", "branch", s.BaseBranch)

					// BRUTAL RECOVERY: If standard recovery fails, delete remote feature branch
					// and let the agent start clean on next iteration.
					if s.JiraTicketID != "" {
						cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
						cmd.Dir = s.Workspace
						if out, err := cmd.Output(); err == nil {
							featureBranch := strings.TrimSpace(string(out))
							if featureBranch != s.BaseBranch && !strings.Contains(featureBranch, "HEAD") {
								fmt.Printf("[%s] BRUTAL RECOVERY: Deleting remote branch %s to clear conflict.\n", s.JiraTicketID, featureBranch)
								_ = gitClient.DeleteRemoteBranch(s.Workspace, "origin", featureBranch)
							}
						}
						// Hard reset to base branch state to ensures clean slate
						fmt.Printf("[%s] Resetting workspace to %s...\n", s.JiraTicketID, s.BaseBranch)
						_ = gitClient.ResetHard(s.Workspace, "origin", s.BaseBranch)
					}

					s.clearSignal("PROJECT_SIGNED_OFF")
					s.EnsureConflictTask()
					s.clearSignal("QA_PASSED")
					s.clearSignal("COMPLETED")
					continue
				}
			}

			// CRITICAL: Guardrail against premature sign-off.
			// Validate that ALL features are actually passing before accepting the sign-off.
			features := s.loadFeatures()
			incompleteFeatures := []string{}
			for _, f := range features {
				if !(f.Passes || f.Status == "done" || f.Status == "implemented") {
					incompleteFeatures = append(incompleteFeatures, f.ID)
				}
			}

			if len(incompleteFeatures) > 0 {

				s.Logger.Warn("premature project sign-off detected", "incomplete_features", incompleteFeatures)

				// Revoke signal
				s.clearSignal("PROJECT_SIGNED_OFF")
				// Also clear QA_PASSED to force re-verification
				s.clearSignal("QA_PASSED")
				// Also clear COMPLETED to force re-check
				s.clearSignal("COMPLETED")

				s.Logger.Info("returning to coding phase")
				continue
			}

			if s.SelectedTaskID != "" {
				fmt.Println("Project signed off. Sub-session exiting.")
				return nil
			}

			// Auto-Merge Logic
			if s.AutoMerge && s.BaseBranch != "" {
				fmt.Printf("Auto-Merge enabled. Preparing to merge changes into base branch: %s\n", s.BaseBranch)

				// 0. COMMIT WORK: Ensure any pending changes are committed before merging
				// We use a more careful commit strategy to avoid re-adding ignored files
				commitCmd := exec.Command("sh", "-c", "git add . && git commit -m 'feat: implemented features for "+s.Project+"' || echo 'Nothing to commit'")
				commitCmd.Dir = s.Workspace
				if out, err := commitCmd.CombinedOutput(); err != nil {
					fmt.Printf("Warning: Failed to auto-commit work: %v\nOutput: %s\n", err, out)
				} else {
					fmt.Printf("Auto-committed work: %s\n", strings.TrimSpace(string(out)))
				}

				fmt.Printf("Merging changes into base branch: %s\n", s.BaseBranch)
				gitClient := git.NewClient()
				// Actually, we are IN the workspace, so we can get current branch name
				// But simpler: checkout BaseBranch -> Merge Previous -> Push

				// 1. Get current branch name
				cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
				cmd.Dir = s.Workspace
				out, err := cmd.Output()
				if err != nil {
					fmt.Printf("Warning: Failed to get current branch for auto-merge: %v\n", err)
				} else {
					featureBranch := strings.TrimSpace(string(out))

					// 2. Checkout Base Branch
					if err := gitClient.Checkout(s.Workspace, s.BaseBranch); err != nil {
						fmt.Printf("Warning: Auto-merge failed (checkout base): %v\n", err)
					} else {
						// 3. Merge Feature Branch
						if err := gitClient.Merge(s.Workspace, featureBranch); err != nil {
							fmt.Printf("Warning: Auto-merge failed (merge): %v\n", err)
							// ENSURE WE ABORT
							_ = gitClient.AbortMerge(s.Workspace)
							_ = gitClient.Recover(s.Workspace)
						} else {
							// 4. Push Base Branch
							if err := gitClient.Push(s.Workspace, s.BaseBranch); err != nil {
								fmt.Printf("Warning: Auto-merge failed (push): %v\n", err)
								// If push fails (likely race), abort the merge locally too so we can retry from clean state
								_ = gitClient.AbortMerge(s.Workspace)
							} else {
								fmt.Printf("Successfully auto-merged %s into %s and pushed.\n", featureBranch, s.BaseBranch)

								// DELETE REMOTE FEATURE BRANCH (Cleanup)
								// This keeps the repo clean and prevents branch accumulation
								fmt.Printf("[%s] Deleting remote feature branch %s...\n", s.Project, featureBranch)
								if err := gitClient.DeleteRemoteBranch(s.Workspace, "origin", featureBranch); err != nil {
									fmt.Printf("[%s] Warning: Failed to delete remote branch: %v\n", s.Project, err)
								}

								// 6. Capture Commit SHA for links
								commitSHA := ""
								shaCmd := exec.Command("git", "rev-parse", "HEAD")
								shaCmd.Dir = s.Workspace
								if shaOut, err := shaCmd.Output(); err == nil {
									commitSHA = strings.TrimSpace(string(shaOut))
								}

								// 7. Transition Jira and notify with commit link
								gitLink := s.RepoURL
								if commitSHA != "" {
									gitLink = fmt.Sprintf("%s/commit/%s", s.RepoURL, commitSHA)
								}
								s.completeJiraTicket(ctx, gitLink)
							}
						}
						// 5. Checkout back to feature branch (nice to have)
						_ = gitClient.Checkout(s.Workspace, featureBranch)
					}
				}
			} else {
				// No auto-merge or no base branch. Just push the feature branch and complete.
				cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
				cmd.Dir = s.Workspace
				if out, err := cmd.Output(); err == nil {
					featureBranch := strings.TrimSpace(string(out))
					// Push current branch
					gitClient := git.NewClient()
					if err := gitClient.Push(s.Workspace, featureBranch); err == nil {
						gitLink := fmt.Sprintf("%s/tree/%s", s.RepoURL, featureBranch)
						s.completeJiraTicket(ctx, gitLink)
					}
				}
			}

			s.Logger.Info("project signed off, running cleaner agent")
			if err := s.runCleanerAgent(ctx); err != nil {
				s.Logger.Error("cleaner agent error", "error", err)
			}
			s.Logger.Info("cleaner agent complete, session finished")
			return nil
		}

		// Global Lifecycle Transitions (QA/Manager) - Main Session Only
		if s.SelectedTaskID == "" {
			if s.hasSignal("QA_PASSED") {
				fmt.Println("QA passed. Running Manager agent for final review...")
				if err := s.runManagerAgent(ctx); err != nil {
					fmt.Printf("Manager agent error: %v\n", err)
					fmt.Println("Manager review failed. Returning to coding phase.")
				} else {
					// Manager approved - create PROJECT_SIGNED_OFF
					if err := s.createSignal("PROJECT_SIGNED_OFF"); err != nil {
						fmt.Printf("Warning: Failed to create PROJECT_SIGNED_OFF: %v\n", err)
					}
					fmt.Println("Manager approved. Project signed off.")
					s.Notifier.Notify(ctx, notify.EventSuccess, fmt.Sprintf("Project %s Signed Off by Manager!", s.Project), s.GetSlackThreadTS())
					continue // Next iteration will run Cleaner
				}
			}

			if s.hasSignal("COMPLETED") {
				// Skip QA if requested (useful for smoketests/verification)
				if s.SkipQA {
					fmt.Println("SkipQA enabled. Bypassing QA agent and Manager review.")
					s.createSignal("PROJECT_SIGNED_OFF")
					s.clearSignal("COMPLETED")
					continue
				}

				fmt.Println("Project marked as COMPLETED. Running QA agent...")
				if err := s.runQAAgent(ctx); err != nil {
					fmt.Printf("QA agent error: %v\n", err)
					// QA failed - clear COMPLETED and continue coding
					s.clearSignal("COMPLETED")
					fmt.Println("QA checks failed. Returning to coding phase.")
				} else {
					// QA passed - create QA_PASSED
					if err := s.createSignal("QA_PASSED"); err != nil {
						fmt.Printf("Warning: Failed to create QA_PASSED signal: %v\n", err)
					}
					fmt.Println("QA checks passed. Moving to Manager review.")
					continue // Next iteration will run Manager
				}
			}
		}

		// Select appropriate prompt and role
		prompt, role, isManager, err := s.SelectPrompt()
		if err != nil {
			fmt.Printf("Error selecting prompt: %v\n", err)
			break
		}

		// Multi-Agent Coding Sprint Delegation
		if role == prompts.CodingAgent && s.MaxAgents > 1 {
			fmt.Printf("Delegating to Multi-Agent Orchestrator (role: %s, max-agents: %d)\n", role, s.MaxAgents)
			orchestrator := NewOrchestrator(s.DBStore, s.Docker, s.Workspace, s.Image, s.Agent, s.Project, s.AgentProvider, s.AgentModel, s.MaxAgents, s.GetSlackThreadTS())
			if err := orchestrator.Run(ctx); err != nil {
				fmt.Printf("Orchestrator sprint failed: %v\n", err)
			}
			// After orchestrator finishes (barrier), we continue the next iteration in the main loop
			if s.checkAutoQA() {
				fmt.Println("Project automatically marked as completed after multi-agent sprint.")
			}
			continue
		}

		// Run iteration using determined prompt
		executionOutput, err := s.RunIteration(ctx, prompt, isManager)

		// Check for Agent/API Error (e.g. 413, Network, etc)
		if err != nil {
			s.Logger.Error("iteration failed", "error", err)
			time.Sleep(5 * time.Second) // Backoff
			continue                    // Retry loop without tripping no-op breaker
		}

		// Circuit Breaker: No-Op Check
		if err := s.checkNoOpBreaker(executionOutput); err != nil {
			fmt.Println(err)
			s.Notifier.Notify(ctx, notify.EventFailure, fmt.Sprintf("Project %s Failed: %v", s.Project, err), s.GetSlackThreadTS())
			s.Notifier.AddReaction(ctx, s.GetSlackThreadTS(), "x")
			return ErrNoOp // Exit loop with error
		}

		// Circuit Breaker: Stalled Progress Check
		passingCount := s.checkFeatures()
		if err := s.checkStalledBreaker(role, passingCount); err != nil {
			telemetry.TrackAgentStall(s.Project)
			fmt.Println(err)
			s.Notifier.Notify(ctx, notify.EventFailure, fmt.Sprintf("Project %s Stalled: %v", s.Project, err), s.GetSlackThreadTS())
			s.Notifier.AddReaction(ctx, s.GetSlackThreadTS(), "x")
			return ErrStalled // Exit loop with error
		}

		// Save agent state periodically (every iteration)
		if err := s.SaveAgentState(); err != nil {
			fmt.Printf("Warning: Failed to save agent state: %v\n", err)
		}

		// Push progress to remote periodically (to ensure visibility in Jira/Git)
		s.pushProgress(ctx)

		time.Sleep(1 * time.Second)
	}

	// Save final agent state before exiting
	// Save final agent state before exiting
	if err := s.SaveAgentState(); err != nil {
		s.Logger.Warn("failed to save final agent state", "error", err)
	}

	s.Logger.Info("session complete")
	return nil
}

// RunIteration executes a single turn of the autonomous agent.
func (s *Session) RunIteration(ctx context.Context, prompt string, isManager bool) (string, error) {
	role := "Agent"
	if isManager {
		role = "Manager"
	}
	s.Logger.Info("agent role selected", "role", role)

	// Send to Agent
	s.Logger.Info("sending prompt to agent")
	var response string
	var err error

	if s.StreamOutput {
		fmt.Print("Agent Response: ")
		response, err = s.Agent.SendStream(ctx, prompt, func(chunk string) {
			fmt.Print(chunk)
		})
		fmt.Println() // Newline after stream
	} else {
		response, err = s.Agent.Send(ctx, prompt)
	}

	if err != nil {
		s.Logger.Error("agent error, retrying", "error", err)
		return "", err
	}

	s.Logger.Info("agent response received", "role", role, "chars", len(response))

	// Repetition Mitigation
	truncated, wasTruncated := TruncateRepetitiveResponse(response)
	if wasTruncated {
		s.Logger.Warn("agent response truncated due to repetition")
		response = truncated + "\n\n[RESPONSE TRUNCATED DUE TO REPETITION DETECTED]"
	}

	// Security Scan
	if s.Scanner != nil {
		findings, err := s.Scanner.Scan(response)
		if err != nil {
			s.Logger.Warn("security scan failed", "error", err)
		} else if len(findings) > 0 {
			s.Logger.Error("security violation detected")
			for _, f := range findings {
				s.Logger.Error("security finding", "type", f.Type, "desc", f.Description, "line", f.Line)
			}
			return "", fmt.Errorf("security violation detected")
		} else {
			s.Logger.Info("security scan passed")
		}
	}

	// Save observation to DB (only if safe)
	if s.DBStore != nil {
		telemetry.TrackDBOp(s.Project)
		if err := s.DBStore.SaveObservation(s.Project, role, response); err != nil {
			s.Logger.Error("failed to save observation to DB", "error", err)
		} else {
			s.Logger.Debug("saved observation to DB")
		}
	}

	// Process Response (Execute Commands & Check Blockers)
	executionOutput, execErr := s.ProcessResponse(ctx, response)

	// Save System Output to DB (Feedback Loop)
	if s.DBStore != nil && executionOutput != "" {
		telemetry.TrackDBOp(s.Project)
		// Use "System" role for tool outputs
		if err := s.DBStore.SaveObservation(s.Project, "System", executionOutput); err != nil {
			s.Logger.Error("failed to save system output to DB", "error", err)
		} else {
			s.Logger.Debug("saved system output to DB")
		}
	}

	return executionOutput, execErr
}

// SelectPrompt determines which prompt to send based on current state.
func (s *Session) SelectPrompt() (string, string, bool, error) {
	// 1. Initializer (Session 1)
	// 1. Initializer Check (Run if feature_list.json is missing or empty)
	// Only for main session (not sub-sessions) and not if ManagerFirst is active on iteration 1
	if s.SelectedTaskID == "" {
		runInitializer := false

		// If ManagerFirst is requested on Iteration 1, we skip Initializer for now
		// (Manager might create it, or we'll loop back and hit this again later if Manager doesn't)
		if s.GetIteration() == 1 && s.ManagerFirst {
			// Manager First: Skip Initializer, go straight to Manager prompt
			// ... (existing logic for ManagerFirst)
			qaReport := "Initial Planning Phase. No code implemented yet."
			prompt, err := prompts.GetPrompt(prompts.ManagerReview, map[string]string{
				"qa_report": qaReport,
			})
			return prompt, prompts.ManagerReview, true, err
		}

		// Check for existing features (DB, Injected, or File)
		features := s.loadFeatures()
		if len(features) > 0 {
			// Features exist, so we don't need to run Initializer.
			// s.loadFeatures() automatically syncs to file if found in DB.
		} else {
			// No features found anywhere. Run Initializer.
			fmt.Println("Feature list not found (in DB, Content, or File). Running Initializer.")
			runInitializer = true
		}

		if runInitializer {
			spec, _ := s.ReadSpec()
			prompt, err := prompts.GetPrompt(prompts.Initializer, map[string]string{
				"spec": spec,
			})
			return prompt, prompts.Initializer, false, err
		}
	}

	// 2. Manager Review (Triggered by file or frequency) - Main Session Only
	if s.SelectedTaskID == "" && (s.GetIteration()%s.ManagerFrequency == 0 || s.hasSignal("TRIGGER_MANAGER")) {
		// Cleanup signal
		s.clearSignal("TRIGGER_MANAGER")

		features := s.loadFeatures()

		qaReport := RunQA(features)

		vars := map[string]string{
			"qa_report": qaReport.String(),
		}

		// Inject Stall Warning if active
		if s.hasSignal("STALLED_WARNING") {
			s.clearSignal("STALLED_WARNING") // Clear after consuming
			vars["stall_warning"] = fmt.Sprintf("CRITICAL WARNING: The Coding Agent has stalled for %d iterations. You must intervene. Review their recent history and provide specific redirection instructions or STOP the project.", s.StalledCount)
		}

		prompt, err := prompts.GetPrompt(prompts.ManagerReview, vars)
		return prompt, prompts.ManagerReview, true, err
	}

	// 3. Coding Agent (Default)
	var historyStr string
	if s.DBStore != nil {
		// Limit history size to prevent context exhaustion (413 errors)
		const MaxHistoryChars = 25000                     // approx 6k tokens, safe for most models
		obs, err := s.DBStore.QueryHistory(s.Project, 20) // Fetch more, but we'll filter by size
		if err == nil {
			var sb strings.Builder

			// Calculate how many observations fit within the limit
			// obs is ordered by created_at DESC (Newest First)
			var includedObs []db.Observation
			currentSize := 0

			for _, o := range obs {
				// Estimate size: Content + Overhead
				size := len(o.Content) + len(o.AgentID) + 20
				if currentSize+size > MaxHistoryChars {
					break
				}
				includedObs = append(includedObs, o)
				currentSize += size
			}

			// Build string in Chronological Order (Oldest -> Newest)
			// includedObs is still [Newest, ..., Oldest-Fitting]
			for i := len(includedObs) - 1; i >= 0; i-- {
				sb.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", includedObs[i].AgentID, includedObs[i].Content))
			}
			historyStr = sb.String()
		}
	}

	vars := map[string]string{
		"history": historyStr,
	}

	// Populate task-specific variables if set
	// 4. Deterministic Task Assignment (User Request: Remove agent reliance on jq)
	// Find the first pending feature and assign it explicitly.
	var assignedFeature *db.Feature
	features := s.loadFeatures() // Refresh from DB/File

	for i := range features {
		if features[i].Status != "done" && !features[i].Passes {
			assignedFeature = &features[i]
			break
		}
	}

	if assignedFeature != nil {
		vars["task_id"] = assignedFeature.ID
		vars["task_description"] = assignedFeature.Description
		vars["exclusive_paths"] = strings.Join(assignedFeature.Dependencies.ExclusiveWritePaths, ", ")
		vars["read_only_paths"] = strings.Join(assignedFeature.Dependencies.ReadOnlyPaths, ", ")

		// s.SelectedTaskID = assignedFeature.ID // DO NOT SET THIS: It prevents Manager interruptions in subsequent turns.
	} else {
		// All done?
		vars["task_id"] = "NONE_ALL_COMPLETE"
		vars["task_description"] = "All features are marked as done/passing. Please run final verification and signal completion."
		vars["exclusive_paths"] = "none"
		vars["read_only_paths"] = "all"
	}
	if s.SelectedTaskID != "" {
		features := s.loadFeatures()
		var target db.Feature
		for _, f := range features {
			if f.ID == s.SelectedTaskID {
				target = f
				break
			}
		}

		if target.ID != "" {
			vars["task_id"] = target.ID

			// Defensive Truncation: Restrict description size to prevent context exhaustion
			desc := target.Description
			const MaxDescriptionChars = 20000
			if len(desc) > MaxDescriptionChars {
				s.Logger.Warn("task description truncated", "original_len", len(desc), "limit", MaxDescriptionChars)
				desc = desc[:MaxDescriptionChars] + "\n\n... [Description Truncated due to size] ..."
			}
			vars["task_description"] = desc

			vars["exclusive_paths"] = strings.Join(target.Dependencies.ExclusiveWritePaths, ", ")
			vars["read_only_paths"] = strings.Join(target.Dependencies.ReadOnlyPaths, ", ")
		} else {
			vars["task_id"] = s.SelectedTaskID
			vars["task_description"] = "No description found in feature_list.json"
			vars["exclusive_paths"] = "None"
			vars["read_only_paths"] = "None"
		}
	} else {
		vars["task_id"] = "Multiple/Not Assigned"
		vars["task_description"] = "Continue implementing pending features in feature_list.json"
		vars["exclusive_paths"] = "All available files"
		vars["read_only_paths"] = "All available files"
	}

	prompt, err := prompts.GetPrompt(prompts.CodingAgent, vars)
	return prompt, prompts.CodingAgent, false, err
}

func (s *Session) loadFeatures() []db.Feature {
	// 1. Try to fetch from DB first (Authoritative source)
	if s.DBStore != nil {
		s.Logger.Info("[DEBUG] Attempting to load features from DB", "project", s.Project)
		content, err := s.DBStore.GetFeatures(s.Project)
		if err != nil {
			s.Logger.Info("[DEBUG] DB GetFeatures error", "error", err)
		}
		if err == nil && content != "" {
			var fl db.FeatureList
			if err := json.Unmarshal([]byte(content), &fl); err == nil {
				s.Logger.Info("loaded features from DB history", "count", len(fl.Features))
				return fl.Features
			}
		}
	} else {
		s.Logger.Info("[DEBUG] No DBStore available for feature lookup")
	}

	// 2. Fallback to FeatureContent (passed from Orchestrator/CLI)
	if s.FeatureContent != "" {
		var fl db.FeatureList
		if err := json.Unmarshal([]byte(s.FeatureContent), &fl); err == nil {
			s.Logger.Info("loaded features from injected content")
			// Sync to DB
			if s.DBStore != nil {
				_ = s.DBStore.SaveFeatures(s.Project, s.FeatureContent)
			}
			return fl.Features
		}
	}

	// 3. Fallback to feature_list.json file (Legacy/Local mode)
	listPath := filepath.Join(s.Workspace, "feature_list.json")
	if data, err := os.ReadFile(listPath); err == nil {
		var fl db.FeatureList
		if err := json.Unmarshal(data, &fl); err == nil {
			s.Logger.Info("loaded features from file", "path", listPath)
			// Sync to DB
			if s.DBStore != nil {
				_ = s.DBStore.SaveFeatures(s.Project, string(data))
			}
			return fl.Features
		}
	}

	return nil
}

func (s *Session) checkCompletion() bool {
	return s.hasSignal("COMPLETED")
}

func (s *Session) hasSignal(name string) bool {
	if s.DBStore == nil {
		return false
	}

	// 1. Check DB (Modern Source)
	val, err := s.DBStore.GetSignal(s.Project, name)
	if err == nil && val == "true" {
		return true
	}

	// 2. Migration: Check Filesystem (Legacy Source)
	path := filepath.Join(s.Workspace, name)
	if _, err := os.Stat(path); err == nil {
		// Found file-based signal.
		// Security Check: Only migrate non-privileged signals from filesystem
		privilegedSignals := map[string]bool{
			"PROJECT_SIGNED_OFF": true,
			"QA_PASSED":          true,
			"COMPLETED":          true,
			"TRIGGER_QA":         true,
			"TRIGGER_MANAGER":    true,
		}

		if privilegedSignals[name] {
			s.Logger.Warn("ignoring filesystem-based privileged signal (must come from DB)", "signal", name)
			return false
		}

		s.Logger.Info("migrating signal from filesystem to DB", "signal", name)
		if err := s.DBStore.SetSignal(s.Project, name, "true"); err != nil {
			s.Logger.Error("failed to migrate signal to DB", "signal", name, "error", err)
			return true // File exists, so logically signal is true even if migration failed
		}
		// Cleanup the file after migration
		os.Remove(path)
		return true
	}

	return false
}

func (s *Session) clearSignal(name string) {
	if s.DBStore != nil {
		s.DBStore.DeleteSignal(s.Project, name)
	}
	// Also ensure file is removed (redundancy)
	path := filepath.Join(s.Workspace, name)
	os.Remove(path)
}

// pushProgress commits and pushes the current state of the workspace to the current branch.
func (s *Session) pushProgress(ctx context.Context) {
	if s.Workspace == "" {
		return
	}

	gitClient := git.NewClient()
	if !gitClient.RepoExists(s.Workspace) {
		return
	}

	// Safety Check: Ensure state files are ignored
	_ = EnsureStateIgnored(s.Workspace)

	// Ensure Host has permissions to read what Agent (Root) wrote (e.g. .git/refs/heads/master)
	if !s.UseLocalAgent {
		if err := s.fixPermissions(ctx); err != nil {
			s.Logger.Warn("failed to fix permissions before push", "error", err)
		}
	}

	s.debugGitState(ctx)

	branch, err := gitClient.CurrentBranch(s.Workspace)
	if err != nil || branch == "" || branch == "main" || branch == "master" {
		// Don't auto-push to main/master
		return
	}

	s.Logger.Info("pushing progress to remote", "branch", branch)

	// Commit any changes (ignore error if nothing to commit)
	msg := fmt.Sprintf("chore: progress update (iteration %d)", s.GetIteration())
	_ = gitClient.Commit(s.Workspace, msg)

	// Workaround: Agent might have run 'git init' which resets HEAD to master in the container
	// We merge master into current branch to capture those commits if they exist
	if branch != "master" && branch != "main" {
		// Try explicit refs to avoid ambiguity or missing short names
		candidates := []string{"refs/heads/master", "refs/heads/main", "master", "main"}
		merged := false
		for _, ref := range candidates {
			if err := gitClient.Merge(s.Workspace, ref); err == nil {
				s.Logger.Info("merged stranded commits from ref", "ref", ref)
				merged = true
				break
			}
		}
		if !merged {
			s.Logger.Debug("no stranded commits merged from master/main")
		}
	}

	// Push progress
	if err := gitClient.Push(s.Workspace, branch); err != nil {
		s.Logger.Warn("failed to push progress", "error", err)
	}
}

// createSignal creates a signal in the DB.
func (s *Session) createSignal(name string) error {
	if s.DBStore == nil {
		return fmt.Errorf("db store not initialized")
	}
	if err := s.DBStore.SetSignal(s.Project, name, "true"); err != nil {
		return err
	}
	s.Logger.Info("created signal", "signal", name)
	return nil
}






// EnsureConflictTask checks if "Resolve Merge Conflicts" task exists, otherwise adds it.
func (s *Session) EnsureConflictTask() {
	if s.DBStore == nil {
		return
	}
	features := s.loadFeatures()
	conflictTaskID := "CONFLICT_RES"
	needsUpdate := false

	// Check if already exists/pending
	for idx, f := range features {
		if f.ID == conflictTaskID {
			if f.Status == "done" || f.Status == "implemented" || f.Passes {
				// Reset it to todo since we have a NEW conflict
				features[idx].Status = "todo"
				features[idx].Passes = false
				needsUpdate = true
			}
			break
		}
	}

	// Add new if not found (needsUpdate loop below handles the save)
	found := false
	for _, f := range features {
		if f.ID == conflictTaskID {
			found = true
			break
		}
	}

	if !found {
		newFeature := db.Feature{
			ID:          conflictTaskID,
			Category:    "Guardrail",
			Priority:    "Critical",
			Description: fmt.Sprintf("Resolve git merge conflicts with branch %s. Files contain conflict markers (<<<< HEAD). Fix them and commit.", s.BaseBranch),
			Status:      "todo",
			Passes:      false,
		}
		features = append(features, newFeature)
		needsUpdate = true
	}

	if needsUpdate {
		fl := db.FeatureList{Features: features}
		data, err := json.Marshal(fl)
		if err == nil {
			_ = s.DBStore.SaveFeatures(s.Project, string(data))
		}
	}
}


// completeJiraTicket performs the final Jira transition, adds a comment with the link, and sends a notification.
func (s *Session) completeJiraTicket(ctx context.Context, gitLink string) {
	if s.JiraClient == nil || (reflect.ValueOf(s.JiraClient).Kind() == reflect.Ptr && reflect.ValueOf(s.JiraClient).IsNil()) || s.JiraTicketID == "" {
		// Not a Jira session, but we still send a notification
		s.Notifier.Notify(ctx, notify.EventProjectComplete, fmt.Sprintf("Project %s is COMPLETE! Git: %s", s.Project, gitLink), s.GetSlackThreadTS())
		return
	}

	fmt.Printf("[%s] Finalizing Jira ticket...\n", s.JiraTicketID)

	// 1. Add Comment with Link
	comment := fmt.Sprintf("RECAC session completed successfully.\n\nGit Link: %s", gitLink)
	if err := s.JiraClient.AddComment(ctx, s.JiraTicketID, comment); err != nil {
		fmt.Printf("[%s] Warning: Failed to add Jira comment: %v\n", s.JiraTicketID, err)
	} else {
		fmt.Printf("[%s] Jira comment added with Git link.\n", s.JiraTicketID)
	}

	// 2. Transition to Done
	// We use "Done" as the default target status, but it could be configurable
	targetStatus := viper.GetString("jira.done_status")
	if targetStatus == "" {
		targetStatus = "Done"
	}

	fmt.Printf("[%s] Transitioning ticket to '%s'...\n", s.JiraTicketID, targetStatus)
	if err := s.JiraClient.SmartTransition(ctx, s.JiraTicketID, targetStatus); err != nil {
		fmt.Printf("[%s] Warning: Failed to transition Jira ticket to %s: %v\n", s.JiraTicketID, targetStatus, err)
	} else {
		fmt.Printf("[%s] Jira ticket transitioned to %s.\n", s.JiraTicketID, targetStatus)
	}

	// 3. Send Notification with Links
	jiraURL := viper.GetString("jira.url")
	if jiraURL == "" {
		jiraURL = os.Getenv("JIRA_URL")
	}
	jiraLink := fmt.Sprintf("%s/browse/%s", jiraURL, s.JiraTicketID)

	notificationMsg := fmt.Sprintf("Project %s is COMPLETE!\n\nJira: %s\nGit: %s", s.Project, jiraLink, gitLink)
	s.Notifier.Notify(ctx, notify.EventProjectComplete, notificationMsg, s.GetSlackThreadTS())
	s.Notifier.AddReaction(ctx, s.GetSlackThreadTS(), "white_check_mark")
}

func (s *Session) debugGitState(ctx context.Context) {
	// Log Branches
	cmd := exec.Command("git", "branch", "-avv")
	cmd.Dir = s.Workspace
	out, _ := cmd.CombinedOutput()
	s.Logger.Info("debug git branches", "output", string(out))

	// Log Status
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = s.Workspace
	out, _ = cmd.CombinedOutput()
	s.Logger.Info("debug git status", "output", string(out))

	// Log Refs
	cmd = exec.Command("git", "show-ref")
	cmd.Dir = s.Workspace
	out, _ = cmd.CombinedOutput()
	s.Logger.Info("debug git refs", "output", string(out))
}
