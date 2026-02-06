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
	"os/user"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/db"
	"recac/internal/docker"
	"recac/internal/security"
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
	SleepFunc                 func(time.Duration) // Function for sleeping (mockable)

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
		SleepFunc:        time.Sleep,
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
		SleepFunc:        time.Sleep,
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



func (s *Session) loadFeatures() []db.Feature {
	// 1. Try to fetch from DB first (Authoritative source)
	var fromDB []db.Feature
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
				fromDB = fl.Features
			}
		}
	} else {
		s.Logger.Info("[DEBUG] No DBStore available for feature lookup")
	}

	// Helper to merge features (DB wins on conflict, but we add new ones)
	mergeFeatures := func(existing []db.Feature, newFeatures []db.Feature) []db.Feature {
		existMap := make(map[string]bool)
		for _, f := range existing {
			existMap[f.Description] = true // Using description as unique key for now as ID might be random
		}
		merged := existing
		for _, f := range newFeatures {
			if !existMap[f.Description] {
				merged = append(merged, f)
			}
		}
		return merged
	}

	// 2. Load Injected Features from Env (Priority Injection)
	envFeaturesJSON := os.Getenv("RECAC_INJECTED_FEATURES")
	if envFeaturesJSON != "" {
		var fl db.FeatureList
		if err := json.Unmarshal([]byte(envFeaturesJSON), &fl); err == nil {
			s.Logger.Info("loaded injected features from env", "count", len(fl.Features))
			// Merge with DB features (Injected features are "System" features, likely critical)
			fromDB = mergeFeatures(fromDB, fl.Features)

			// Persist the merged state immediately
			if s.DBStore != nil {
				// Re-serialize
				finalList := db.FeatureList{
					ProjectName: s.Project, // Reuse project ID/Name
					Features:    fromDB,
				}
				if data, err := json.Marshal(finalList); err == nil {
					_ = s.DBStore.SaveFeatures(s.Project, string(data))
				}
			}
		}
	}

	if len(fromDB) > 0 {
		// Sync DB -> Filesystem (Ensure agents see what's in DB, e.g. Injected Features)
		listPath := filepath.Join(s.Workspace, "feature_list.json")
		if _, err := os.Stat(listPath); os.IsNotExist(err) {
			finalList := db.FeatureList{
				ProjectName: s.Project,
				Features:    fromDB,
			}
			if data, err := json.MarshalIndent(finalList, "", "  "); err == nil {
				s.Logger.Info("syncing features from DB to feature_list.json", "path", listPath)
				_ = os.WriteFile(listPath, data, 0644)
			}
		}
		return fromDB
	}

	// 3. Fallback to FeatureContent (passed from Orchestrator/CLI legacy)
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

	// 4. Fallback to feature_list.json file (Legacy/Local mode)
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





