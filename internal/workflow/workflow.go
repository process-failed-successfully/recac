package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/docker"
	"recac/internal/git"
	"recac/internal/jira"
	"recac/internal/runner"
	"recac/internal/telemetry"

	"github.com/spf13/viper"
)

// SessionConfig holds all parameters for a RECAC session
type SessionConfig struct {
	Goal              string
	ProjectPath       string
	ProjectName       string
	IsMock            bool
	MaxIterations     int
	ManagerFrequency  int
	MaxAgents         int
	TaskMaxIterations int
	Detached          bool
	SessionName       string
	JiraEpicKey       string
	AllowDirty        bool
	Stream            bool
	AutoMerge         bool
	SkipQA            bool
	ManagerFirst      bool
	Debug             bool
	JiraClient        *jira.Client
	JiraTicketID      string
	RepoURL           string
	Image             string
	Provider          string
	Model             string
	Cleanup           bool
	Summary           string
	Description       string
	Logger            *slog.Logger
	CommandPrefix     []string // Command arguments to prepend (e.g. "start")
	SessionManager    ISessionManager
}

// ProcessDirectTask handles a coding session from a direct repository and task description
var ProcessDirectTask = func(ctx context.Context, cfg SessionConfig) error {
	// Initialize Logger
	if cfg.Logger == nil {
		cfg.Logger = telemetry.NewLogger(cfg.Debug, "", false)
	}
	logger := cfg.Logger
	if cfg.SessionName == "" {
		cfg.SessionName = "direct-task"
	}

	workID := cfg.SessionName
	if cfg.JiraTicketID != "" {
		workID = cfg.JiraTicketID
	}

	logger.Info("Starting direct task session", "repo", cfg.RepoURL, "summary", cfg.Summary, "id", workID)

	// Setup Workspace
	timestamp := time.Now().Format("20060102-150405")

	if cfg.ProjectPath == "" {
		var err error
		cfg.ProjectPath, err = os.MkdirTemp("", "recac-direct-*")
		if err != nil {
			logger.Error("Error creating temp workspace", "error", err)
			return err
		}
	}

	gitClient := git.NewClient()
	if _, err := cmdutils.SetupWorkspace(ctx, gitClient, cfg.RepoURL, cfg.ProjectPath, workID, "", timestamp); err != nil {
		logger.Error("Error: Failed to setup workspace", "error", err)
		return err
	}

	// Force task context: Overwrite app_spec.txt
	if cfg.Summary != "" || cfg.Description != "" {
		specContent := fmt.Sprintf("# Task Summary: %s\n\n%s", cfg.Summary, cfg.Description)
		specPath := filepath.Join(cfg.ProjectPath, "app_spec.txt")
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			logger.Error("Error writing app_spec.txt", "error", err)
			return err
		}

		logger.Info("Refreshed workspace context from task description")
	}

	// Run Workflow
	if err := RunWorkflow(ctx, cfg); err != nil {
		logger.Error("Session failed", "error", err)
		return err
	} else {
		logger.Info("Session completed successfully")
		return nil
	}
}

// ProcessJiraTicket handles the Jira-specific workflow and then runs the project session
var ProcessJiraTicket = func(ctx context.Context, jiraTicketID string, jClient *jira.Client, cfg SessionConfig, ignoredBlockers map[string]bool) error {
	// Initialize Ticket Logger
	if cfg.Logger == nil {
		cfg.Logger = telemetry.NewLogger(cfg.Debug, "", false)
	}
	logger := cfg.Logger.With("ticket_id", jiraTicketID)
	cfg.Logger = logger // Pass it down

	// 2. Fetch Ticket
	ticket, err := jClient.GetTicket(ctx, jiraTicketID)
	if err != nil {
		logger.Error("Error fetching Jira ticket", "error", err)
		return err
	}

	// 2a. Check for Blockers
	blockers := jClient.GetBlockers(ticket)
	if len(blockers) > 0 {
		var effectiveBlockers []string
		for _, b := range blockers {
			// Format is "KEY (Status)"
			parts := strings.Split(b, " (")
			key := parts[0]
			if !ignoredBlockers[key] {
				effectiveBlockers = append(effectiveBlockers, b)
			}
		}

		if len(effectiveBlockers) > 0 {
			logger.Info("SKIPPING: Ticket is blocked", "blockers", strings.Join(effectiveBlockers, ", "))
			return nil // Not an error, just skipped
		}
	}

	// Extract details
	fields, ok := ticket["fields"].(map[string]interface{})
	if !ok {
		logger.Error("Error: Invalid ticket format (missing fields)")
		return fmt.Errorf("invalid ticket format")
	}
	summary, _ := fields["summary"].(string)
	description := jClient.ParseDescription(ticket)

	// Epic Detection
	if parent, ok := fields["parent"].(map[string]interface{}); ok {
		if parentKey, ok := parent["key"].(string); ok {
			cfg.JiraEpicKey = parentKey
			logger.Info("Detected parent Epic", "epic_key", cfg.JiraEpicKey)
		}
	}

	logger.Info("Ticket Found", "summary", summary)

	timestamp := time.Now().Format("20060102-150405")
	var tempWorkspace string

	if cfg.ProjectPath != "" {
		tempWorkspace = cfg.ProjectPath
		// Ensure directory exists
		if err := os.MkdirAll(tempWorkspace, 0755); err != nil {
			logger.Error("Error creating/verifying workspace path", "path", tempWorkspace, "error", err)
			return err
		}
		logger.Info("Using provided workspace path", "path", tempWorkspace)
	} else {
		pattern := fmt.Sprintf("recac-jira-%s-%s-*", jiraTicketID, timestamp)
		tempWorkspace, err = os.MkdirTemp("", pattern)
		if err != nil {
			logger.Error("Error creating temp workspace", "error", err)
			return err
		}
	}

	repoURL := cfg.RepoURL
	if repoURL == "" {
		matches := jira.RepoRegex.FindStringSubmatch(description)
		if len(matches) <= 1 {
			logger.Error("Error: No repository URL found in ticket description (Repo: https://...)")
			return fmt.Errorf("no repo url found")
		}
		repoURL = strings.TrimSuffix(matches[1], ".git")
		repoURL = strings.TrimSuffix(repoURL, "/")
		logger.Info("Found repository URL in ticket", "repo_url", repoURL)
	} else {
		logger.Info("Using provided repository URL", "repo_url", repoURL)
	}

	gitClient := git.NewClient()
	if _, err := cmdutils.SetupWorkspace(ctx, gitClient, repoURL, tempWorkspace, jiraTicketID, cfg.JiraEpicKey, timestamp); err != nil {
		logger.Error("Error: Failed to setup workspace", "error", err)
		return err
	}

	// 5. Create app_spec.txt
	specContent := fmt.Sprintf("# Jira Ticket: %s\n# Summary: %s\n\n%s", jiraTicketID, summary, description)
	specPath := filepath.Join(tempWorkspace, "app_spec.txt")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		logger.Error("Error writing app_spec.txt", "error", err)
		return err
	}

	logger.Info("Workspace created", "path", tempWorkspace)

	// Auto-cleanup
	if cfg.Cleanup {
		defer func() {
			logger.Info("Cleaning up workspace", "path", tempWorkspace)
			if err := os.RemoveAll(tempWorkspace); err != nil {
				logger.Error("Failed to cleanup workspace", "path", tempWorkspace, "error", err)
			}
		}()
	}

	// 5. Transition Ticket Status
	transition := viper.GetString("jira.transition")
	if transition == "" {
		transition = "In Progress"
	}

	logger.Info("Transitioning ticket", "transition", transition)
	if err := jClient.SmartTransition(ctx, jiraTicketID, transition); err != nil {
		logger.Warn("Failed to transition Jira ticket", "error", err)
	} else {
		logger.Info("Jira ticket status updated")
	}

	// Update configuration for the session run
	cfg.ProjectPath = tempWorkspace
	if cfg.SessionName == "" {
		cfg.SessionName = jiraTicketID
	}
	cfg.JiraClient = jClient
	cfg.JiraTicketID = jiraTicketID
	cfg.RepoURL = repoURL

	// Run Workflow
	if err := RunWorkflow(ctx, cfg); err != nil {
		logger.Error("Session failed", "error", err)
		return err
	} else {
		logger.Info("Session completed successfully")
		return nil
	}
}

// ISessionManager defines the interface for session management.
type ISessionManager interface {
	StartSession(name, goal string, command []string, cwd string) (*runner.SessionState, error)
}

// Statically assert that the real session manager implements our interface.
var _ ISessionManager = (*runner.SessionManager)(nil)

// Allow mocking SessionManager creation
var NewSessionManagerFunc = func() (ISessionManager, error) {
	return runner.NewSessionManager()
}

// Allow mocking Session creation
var NewSessionFunc = runner.NewSession

// RunWorkflow handles the execution of a single project session (local or Jira-based)
var RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
	// Handle detached mode
	if cfg.Detached {
		if cfg.SessionName == "" {
			return fmt.Errorf("--name is required when using --detached")
		}

		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %v", err)
		}

		executable, err = filepath.EvalSymlinks(executable)
		if err != nil {
			executable, _ = filepath.Abs(executable)
		} else {
			executable, _ = filepath.Abs(executable)
		}

		// Verify executable
		if stat, err := os.Stat(executable); err != nil || stat.Mode()&0111 == 0 {
			// Fallback to recac-app in CWD
			cwd, _ := os.Getwd()
			fallback := filepath.Join(cwd, "recac-app")
			if stat2, err2 := os.Stat(fallback); err2 == nil && stat2.Mode()&0111 != 0 {
				executable = fallback
			} else {
				return fmt.Errorf("executable not found or not executable at %s", executable)
			}
		}

		// Construct command
		command := []string{executable}
		if len(cfg.CommandPrefix) > 0 {
			command = append(command, cfg.CommandPrefix...)
		} else if filepath.Base(executable) == "recac" {
			// Backward compatibility: if running 'recac' binary, assume 'start' needed if not provided
			command = append(command, "start")
		}
		// NOTE: In detached mode, we need to call 'agent' binary now.
		// However, we are reusing 'recac start' logic here possibly.
		// If we are moving to 'agent' binary, this recursion needs to call 'agent'.
		// We can fix this by ensuring executable calls "start" only if it's the old 'recac' binary.
		// Or if 'agent' binary, it doesn't need "start" subcommand if it's root.
		// We'll assume the command args need to be adapted.
		// For now, we reuse the exact arguments logic but keep in mind this might need adjustment for new binary.
		// Since we are compiling 'recac' still, this logic persists.
		// For 'agent' binary specific usage, we might override this.
		// Let's assume we call "start" if it's recac, and maybe no subcommand if 'agent'?
		// Actually, if we use cobra in 'agent' binary with 'start' cmd, it works universally.

		if cfg.ProjectPath != "" {
			command = append(command, "--path", cfg.ProjectPath)
		}
		if cfg.IsMock {
			command = append(command, "--mock")
		}
		if cfg.MaxIterations != 20 {
			command = append(command, "--max-iterations", fmt.Sprintf("%d", cfg.MaxIterations))
		}
		if cfg.ManagerFrequency != 5 {
			command = append(command, "--manager-frequency", fmt.Sprintf("%d", cfg.ManagerFrequency))
		}
		if cfg.TaskMaxIterations != 10 {
			command = append(command, "--task-max-iterations", fmt.Sprintf("%d", cfg.TaskMaxIterations))
		}
		if cfg.AllowDirty {
			command = append(command, "--allow-dirty")
		}

		projectPath := cfg.ProjectPath
		if projectPath == "" {
			projectPath = "."
		}

		sm := cfg.SessionManager
		if sm == nil {
			var err error
			sm, err = NewSessionManagerFunc()
			if err != nil {
				return fmt.Errorf("failed to create session manager: %v", err)
			}
		}

		session, err := sm.StartSession(cfg.SessionName, cfg.Goal, command, projectPath)
		if err != nil {
			return fmt.Errorf("failed to start detached session: %v", err)
		}

		fmt.Printf("Session '%s' started in background (PID: %d)\n", cfg.SessionName, session.PID)
		fmt.Printf("Log file: %s\n", session.LogFile)
		return nil
	}

	// Mock mode
	if cfg.IsMock {
		fmt.Printf("[%s] Starting in MOCK MODE\n", cfg.SessionName)
		dockerCli, _ := docker.NewMockClient()
		agentClient := agent.NewMockAgent()

		projectPath := cfg.ProjectPath
		if projectPath == "" {
			projectPath = "/tmp/recac-mock-workspace"
		}

		projectName := cfg.ProjectName
		if projectName == "" {
			projectName = "mock-project"
		}

		session := NewSessionFunc(dockerCli, agentClient, projectPath, cfg.Image, projectName, cfg.Provider, cfg.Model, cfg.MaxAgents)
		if cfg.Logger != nil {
			session.Logger = cfg.Logger
		}
		session.MaxIterations = cfg.MaxIterations
		session.TaskMaxIterations = cfg.TaskMaxIterations
		session.ManagerFrequency = cfg.ManagerFrequency
		session.StreamOutput = cfg.Stream
		session.AutoMerge = cfg.AutoMerge
		session.SkipQA = cfg.SkipQA
		session.ManagerFirst = cfg.ManagerFirst

		if cfg.JiraEpicKey != "" {
			session.BaseBranch = fmt.Sprintf("agent-epic/%s", cfg.JiraEpicKey)
		}

		if err := session.Start(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		return session.RunLoop(ctx)
	}

	// Normal mode
	fmt.Printf("[%s] Starting RECAC session...\n", cfg.SessionName)

	projectPath := cfg.ProjectPath
	if projectPath == "" {
		projectPath = "."
	}

	// Pre-flight check
	if !cfg.AllowDirty {
		cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
		cmd.Dir = projectPath
		if err := cmd.Run(); err == nil {
			cmd := exec.Command("git", "status", "--porcelain")
			cmd.Dir = projectPath
			output, _ := cmd.Output()
			if len(output) > 0 {
				return fmt.Errorf("uncommitted changes detected in %s. Run with --allow-dirty to bypass", projectPath)
			}
		}
	}

	projectName := cfg.ProjectName
	if projectName == "" {
		projectName = filepath.Base(projectPath)
		if projectName == "." || projectName == "/" {
			cwd, _ := os.Getwd()
			projectName = filepath.Base(cwd)
		}
	}

	if cfg.SessionName == "" {
		cfg.SessionName = projectName
	}

	var dockerCli *docker.Client
	var err error
	dockerCli, err = docker.NewClient(projectName)
	if err != nil {
		fmt.Printf("Warning: Failed to initialize Docker client: %v. Proceeding in restricted mode.\n", err)
		dockerCli = nil
	}

	provider := cfg.Provider
	model := cfg.Model
	agentClient, err := cmdutils.GetAgentClient(ctx, provider, model, projectPath, projectName)
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %v", err)
	}

	session := NewSessionFunc(dockerCli, agentClient, projectPath, cfg.Image, projectName, provider, model, cfg.MaxAgents)
	if cfg.Logger != nil {
		session.Logger = cfg.Logger
	}
	session.MaxIterations = cfg.MaxIterations
	session.TaskMaxIterations = cfg.TaskMaxIterations
	session.ManagerFrequency = cfg.ManagerFrequency
	session.ManagerFirst = cfg.ManagerFirst
	session.StreamOutput = cfg.Stream
	session.AutoMerge = cfg.AutoMerge
	session.SkipQA = cfg.SkipQA
	session.JiraClient = cfg.JiraClient
	session.JiraTicketID = cfg.JiraTicketID
	session.RepoURL = cfg.RepoURL

	if cfg.JiraEpicKey != "" {
		session.BaseBranch = fmt.Sprintf("agent-epic/%s", cfg.JiraEpicKey)
	}

	// State Management
	if session.StateManager != nil {
		maxTokens := viper.GetInt("agent.max_tokens")
		if maxTokens == 0 {
			maxTokens = 128000
		}
		session.InitializeAgentState(maxTokens)

		if geminiClient, ok := agentClient.(*agent.GeminiClient); ok {
			geminiClient.WithStateManager(session.StateManager)
		} else if openAIClient, ok := agentClient.(*agent.OpenAIClient); ok {
			openAIClient.WithStateManager(session.StateManager)
		} else if openRouterClient, ok := agentClient.(*agent.OpenRouterClient); ok {
			openRouterClient.WithStateManager(session.StateManager)
		}
	}

	if err := session.Start(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	return session.RunLoop(ctx)
}
