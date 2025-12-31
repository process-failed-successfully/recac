package runner

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/agent/prompts"
	"recac/internal/db"
	"recac/internal/docker"
	"recac/internal/security"
	"regexp"
	"strings"
	"time"
)

var ErrBlocker = errors.New("blocker detected")

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
	ManagerAgent agent.Agent
	CleanerAgent agent.Agent
	QAAgent      agent.Agent

	// Circuit Breaker State
	LastFeatureCount int // Number of passing features last time we checked
	StalledCount     int // Number of iterations without feature progress
	NoOpCount        int // Number of iterations without executed commands
}

func NewSession(d DockerClient, a agent.Agent, workspace, image string) *Session {
	// Default agent state file path in workspace
	agentStateFile := filepath.Join(workspace, ".agent_state.json")
	stateManager := agent.NewStateManager(agentStateFile)

	// Initialize DB Store
	// Initialize DB Store
	dbPath := filepath.Join(workspace, ".recac.db")
	var dbStore db.Store
	if sqliteStore, err := db.NewSQLiteStore(dbPath); err != nil {
		fmt.Printf("Warning: Failed to initialize SQLite store: %v\n", err)
	} else {
		dbStore = sqliteStore
	}

	// Initialize Security Scanner
	scanner := security.NewRegexScanner()

	return &Session{
		Docker:           d,
		Agent:            a,
		Workspace:        workspace,
		Image:            image,
		SpecFile:         "app_spec.txt",
		MaxIterations:    20, // Default
		ManagerFrequency: 5,  // Default
		AgentStateFile:   agentStateFile,
		StateManager:     stateManager,
		DBStore:          dbStore,
		Scanner:          scanner,
	}
}

// NewSessionWithStateFile creates a session with a specific agent state file (for restoring sessions)
func NewSessionWithStateFile(d DockerClient, a agent.Agent, workspace, image, agentStateFile string) *Session {
	stateManager := agent.NewStateManager(agentStateFile)

	// Initialize DB Store
	// Initialize DB Store
	dbPath := filepath.Join(workspace, ".recac.db")
	var dbStore db.Store
	if sqliteStore, err := db.NewSQLiteStore(dbPath); err != nil {
		fmt.Printf("Warning: Failed to initialize SQLite store: %v\n", err)
	} else {
		dbStore = sqliteStore
	}

	// Initialize Security Scanner
	scanner := security.NewRegexScanner()

	return &Session{
		Docker:           d,
		Agent:            a,
		Workspace:        workspace,
		Image:            image,
		SpecFile:         "app_spec.txt",
		MaxIterations:    20, // Default
		ManagerFrequency: 5,  // Default
		AgentStateFile:   agentStateFile,
		StateManager:     stateManager,
		DBStore:          dbStore,
		Scanner:          scanner,
	}
}

// LoadAgentState loads agent state from disk if it exists
func (s *Session) LoadAgentState() error {
	if s.StateManager == nil {
		return nil // No state manager configured
	}

	state, err := s.StateManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load agent state: %w", err)
	}

	// If state has memory/history, we can use it to restore context
	// For now, we just ensure the state is loaded (agents that support StateManager will use it)
	if len(state.Memory) > 0 {
		fmt.Printf("Loaded agent state: %d memory items, %d history messages\n", len(state.Memory), len(state.History))
	}

	// Log token usage if available
	if state.TokenUsage.TotalTokens > 0 {
		fmt.Printf("Token usage: total=%d (prompt=%d, response=%d), current=%d/%d, truncations=%d\n",
			state.TokenUsage.TotalTokens,
			state.TokenUsage.TotalPromptTokens,
			state.TokenUsage.TotalResponseTokens,
			state.CurrentTokens,
			state.MaxTokens,
			state.TokenUsage.TruncationCount)
	}

	return nil
}

// InitializeAgentState initializes agent state with max_tokens from config
func (s *Session) InitializeAgentState(maxTokens int) error {
	if s.StateManager == nil {
		return nil // No state manager configured
	}

	return s.StateManager.InitializeState(maxTokens)
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
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read spec file: %w", err)
	}
	return string(content), nil
}

// Start initializes the session environment (Docker container).
func (s *Session) Start(ctx context.Context) error {
	fmt.Printf("Initializing session with image: %s\n", s.Image)

	// Check Docker Daemon
	if err := s.Docker.CheckDaemon(ctx); err != nil {
		return fmt.Errorf("docker check failed: %w", err)
	}

	// Read Spec
	spec, err := s.ReadSpec()
	if err != nil {
		fmt.Printf("Warning: Failed to read spec: %v\n", err)
	} else {
		fmt.Printf("Loaded spec: %d bytes\n", len(spec))
	}

	// Ensure Image is ready
	if err := s.ensureImage(ctx); err != nil {
		fmt.Printf("Warning: Failed to ensure image %s: %v. Attempting to proceed anyway...\n", s.Image, err)
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

	// 1.5 Inject agent-bridge
	if err := s.injectAgentBridge(); err != nil {
		fmt.Printf("Warning: Failed to inject agent-bridge: %v\n", err)
	}

	// Run Container
	id, err := s.Docker.RunContainer(ctx, s.Image, s.Workspace, extraBinds, containerUser)

	s.ContainerID = id
	fmt.Printf("Container started successfully. ID: %s\n", id)
	return nil
}

func (s *Session) ensureImage(ctx context.Context) error {
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

	// 2. If using default image name, ensure it's built from our embedded template
	if s.Image == "recac-agent:latest" {
		exists, err := s.Docker.ImageExists(ctx, s.Image)
		if err != nil {
			return fmt.Errorf("failed to check image existence: %w", err)
		}

		if !exists {
			fmt.Println("Default agent image 'recac-agent:latest' not found. Building from template...")

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
				return fmt.Errorf("failed to build default agent image: %w", err)
			}
			fmt.Printf("Default agent image built successfully: %s\n", newID)
		}
	}

	return nil
}

// Stop cleans up the Docker container.
func (s *Session) Stop(ctx context.Context) error {
	if s.DBStore != nil {
		if err := s.DBStore.Close(); err != nil {
			fmt.Printf("Warning: Failed to close DB store: %v\n", err)
		}
	}

	if s.ContainerID == "" {
		return nil // No container to clean up
	}

	fmt.Printf("Stopping container: %s\n", s.ContainerID)
	if err := s.Docker.StopContainer(ctx, s.ContainerID); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	s.ContainerID = ""
	fmt.Println("Container stopped successfully")

	// Cleanup agent-bridge
	if err := s.cleanupAgentBridge(); err != nil {
		fmt.Printf("Warning: Failed to cleanup agent-bridge: %v\n", err)
	}

	return nil
}

// RunLoop executes the autonomous agent loop.
func (s *Session) RunLoop(ctx context.Context) error {
	fmt.Println("\n=== Entering Autonomous Run Loop ===")

	// Load agent state if it exists (for session restoration)
	if err := s.LoadAgentState(); err != nil {
		fmt.Printf("Warning: Failed to load agent state: %v\n", err)
		// Continue anyway - state will be created on first save
	}

	// Load DB history if available
	if s.DBStore != nil {
		history, err := s.DBStore.QueryHistory(5)
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
		}
	}

	// Ensure cleanup on exit (defer cleanup)
	defer func() {
		if s.ContainerID != "" {
			fmt.Printf("Cleaning up container: %s\n", s.ContainerID)
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := s.Docker.StopContainer(cleanupCtx, s.ContainerID); err != nil {
				fmt.Printf("Warning: Failed to cleanup container: %v\n", err)
			} else {
				fmt.Println("Container cleaned up successfully")
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
		if s.MaxIterations > 0 && s.Iteration >= s.MaxIterations {
			fmt.Printf("Reached max iterations (%d). Stopping.\n", s.MaxIterations)
			break
		}

		s.Iteration++
		fmt.Printf("\n--- Iteration %d ---\n", s.Iteration)

		// Ensure feature list is synced and mirror is up to date
		_ = s.loadFeatures()

		// Handle Lifecycle Role Transitions (Agent-QA-Manager-Cleaner workflow)
		// Prioritize these checks at the beginning of the iteration
		if s.hasSignal("PROJECT_SIGNED_OFF") {
			fmt.Println("Project signed off. Running Cleaner agent...")
			if err := s.runCleanerAgent(ctx); err != nil {
				fmt.Printf("Cleaner agent error: %v\n", err)
			}
			fmt.Println("Cleaner agent complete. Session finished.")
			break
		}

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
				continue // Next iteration will run Cleaner
			}
		}

		if s.hasSignal("COMPLETED") {
			fmt.Println("Project marked as COMPLETED. Running QA agent...")
			if err := s.runQAAgent(ctx); err != nil {
				fmt.Printf("QA agent error: %v\n", err)
				// QA failed - clear COMPLETED and continue coding
				s.clearSignal("COMPLETED")
				fmt.Println("QA checks failed. Returning to coding phase.")
			} else {
				// QA passed - create QA_PASSED
				if err := s.createSignal("QA_PASSED"); err != nil {
					fmt.Printf("Warning: Failed to create QA_PASSED: %v\n", err)
				}
				fmt.Println("QA checks passed. Moving to Manager review.")
				continue // Next iteration will run Manager
			}
		}

		// Select Prompt
		prompt, isManager, err := s.SelectPrompt()
		if err != nil {
			return fmt.Errorf("failed to select prompt: %w", err)
		}

		role := "Agent"
		if isManager {
			role = "Manager"
		}
		fmt.Printf("Role: %s\n", role)

		// Send to Agent
		fmt.Println("Sending prompt to agent...")
		var response string

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
			fmt.Printf("Agent error: %v. Retrying in next iteration...\n", err)
			time.Sleep(2 * time.Second)
			continue
		}

		fmt.Printf("Response received (%d chars).\n", len(response))

		// Repetition Mitigation
		truncated, wasTruncated := TruncateRepetitiveResponse(response)
		if wasTruncated {
			fmt.Println("WARNING: Agent response was truncated due to excessive repetition.")
			response = truncated + "\n\n[RESPONSE TRUNCATED DUE TO REPETITION DETECTED]"
		}

		// Security Scan
		if s.Scanner != nil {
			findings, err := s.Scanner.Scan(response)
			if err != nil {
				fmt.Printf("Warning: Security scan failed: %v\n", err)
			} else if len(findings) > 0 {
				fmt.Println("CRITICAL: Security violation detected in agent response!")
				for _, f := range findings {
					fmt.Printf("  - %s: %s (Line %d)\n", f.Type, f.Description, f.Line)
				}
				fmt.Println("Blocking response execution.")

				// Force a retry or feedback loop here?
				// For now, we continue but treat it as a failure to execute.
				// In a real loop, we would append this to history and ask for correction.
				continue
			} else {
				fmt.Println("Security scan passed.")
			}
		}

		// Save observation to DB (only if safe)
		if s.DBStore != nil {
			if err := s.DBStore.SaveObservation(role, response); err != nil {
				fmt.Printf("Warning: Failed to save observation to DB: %v\n", err)
			} else {
				fmt.Println("Saved observation to DB.")
			}
		}

		// Process Response (Execute Commands & Check Blockers)
		executionOutput, err := s.ProcessResponse(ctx, response)
		if errors.Is(err, ErrBlocker) {
			return nil // Clean exit
		}
		if err != nil {
			fmt.Printf("Error processing response: %v\n", err)
		} else if executionOutput != "" {
			// Save execution output (System role)
			if s.DBStore != nil {
				if err := s.DBStore.SaveObservation("System", executionOutput); err != nil {
					fmt.Printf("Warning: Failed to save system observation: %v\n", err)
				}
			}
		}

		// Check for Auto-QA Trigger
		if s.checkAutoQA() {
			if err := s.createSignal("COMPLETED"); err != nil {
				fmt.Printf("Warning: Failed to create COMPLETED signal: %v\n", err)
			}
			fmt.Println("Auto-triggered QA: All features passing.")
		}

		// Check for Manual QA Trigger (from agent-bridge)
		if s.hasSignal("TRIGGER_QA") {
			s.clearSignal("TRIGGER_QA")
			if err := s.createSignal("COMPLETED"); err != nil {
				fmt.Printf("Warning: Failed to create COMPLETED signal: %v\n", err)
			}
			fmt.Println("Manual QA trigger received.")
		}

		// Check for Manager Trigger (from agent-bridge)
		if s.hasSignal("TRIGGER_MANAGER") {
			// Just ensure manager gets triggered next loop via TRIGGER_MANAGER logic in SelectPrompt
			// Actually SelectPrompt logic checks TRIGGER_MANAGER, so we just need to leave it set.
			// But wait, SelectPrompt clears it. So we are good.
			fmt.Println("Manual Manager trigger received.")
		}

		// Circuit Breaker: No-Op Check
		if err := s.checkNoOpBreaker(executionOutput); err != nil {
			fmt.Println(err)
			return nil // Exit loop
		}

		// Circuit Breaker: Stalled Progress Check
		passingCount := s.checkFeatures()
		if err := s.checkStalledBreaker(role, passingCount); err != nil {
			fmt.Println(err)
			return nil // Exit loop
		}

		// Save agent state periodically (every iteration)
		if err := s.SaveAgentState(); err != nil {
			fmt.Printf("Warning: Failed to save agent state: %v\n", err)
		}

		time.Sleep(1 * time.Second)
	}

	// Save final agent state before exiting
	if err := s.SaveAgentState(); err != nil {
		fmt.Printf("Warning: Failed to save final agent state: %v\n", err)
	}

	fmt.Println("\n=== Session Complete ===")
	return nil
}

// SelectPrompt determines which prompt to send based on current state.
func (s *Session) SelectPrompt() (string, bool, error) {
	// 1. Initializer (Session 1)
	// 1. Initializer (Session 1)
	if s.Iteration == 1 {
		if s.ManagerFirst {
			// Manager First: Skip Initializer, go straight to Manager prompt
			// We simulate a "trigger" so the prompt logic below picks it up as a Manager turn
			// Actually, easier to return ManagerReview prompt directly here, but need args.
			// Manager Review usually takes a QA report. For first run, maybe just the spec?
			// The reference says "Run the Manager Agent before the first coding session".
			// Let's use the ManagerReview prompt but maybe passing "Initial Planning Phase" as context if possible.
			// Currently ManagerReview takes {qa_report}.
			// Let's construct a pseudo-report.
			qaReport := "Initial Planning Phase. No code implemented yet."
			prompt, err := prompts.GetPrompt(prompts.ManagerReview, map[string]string{
				"qa_report": qaReport,
			})
			return prompt, true, err
		}

		spec, _ := s.ReadSpec()
		featuresPath := filepath.Join(s.Workspace, "feature_list.json")

		// 1. Check if file exists
		if _, err := os.Stat(featuresPath); err == nil {
			// File exists, let's see if it's loadable
			features := s.loadFeatures()
			if features != nil {
				fmt.Println("Feature list found. Skipping Initializer.")
			} else {
				// File exists but is invalid JSON (e.g. empty or corrupted)
				// We SHOULD NOT run Initializer here as it would overwrite work.
				// Better to stop and let the human fix the JSON.
				return "", false, fmt.Errorf("feature_list.json exists but is invalid or empty. Please fix the file before continuing")
			}
		} else {
			// File does NOT exist. Run Initializer.
			prompt, err := prompts.GetPrompt(prompts.Initializer, map[string]string{
				"spec": spec,
			})
			return prompt, false, err
		}
	}

	// 2. Manager Review (Triggered by file or frequency)
	if s.Iteration%s.ManagerFrequency == 0 || s.hasSignal("TRIGGER_MANAGER") {
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
		return prompt, true, err
	}

	// 3. Coding Agent (Default)
	var historyStr string
	if s.DBStore != nil {
		obs, err := s.DBStore.QueryHistory(15) // Limit 15 to capture context
		if err == nil {
			var sb strings.Builder
			for i := len(obs) - 1; i >= 0; i-- {
				sb.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", obs[i].AgentID, obs[i].Content))
			}
			historyStr = sb.String()
		}
	}

	prompt, err := prompts.GetPrompt(prompts.CodingAgent, map[string]string{
		"history": historyStr,
	})
	return prompt, false, err
}

func (s *Session) loadFeatures() []db.Feature {
	filePath := filepath.Join(s.Workspace, "feature_list.json")

	// 1. Try to fetch from DB first (Authoritative source)
	if s.DBStore != nil {
		content, err := s.DBStore.GetFeatures()
		if err == nil && content != "" {
			var fl db.FeatureList
			if err := json.Unmarshal([]byte(content), &fl); err == nil {
				// Authoritative data found in DB - Sync BACK to file to restore it (effectively read-only/mirror)
				s.syncFeatureFile(fl)
				return fl.Features
			}
		}
	}

	// 2. Fallback to file (First run or recovery)
	data, err := os.ReadFile(filePath)
	if err == nil {
		var fl db.FeatureList
		if err := json.Unmarshal(data, &fl); err == nil {
			// Successfully read valid JSON from file - Sync to DB if DB is empty
			if s.DBStore != nil {
				_ = s.DBStore.SaveFeatures(string(data))
			}
			return fl.Features
		}
	}

	return nil
}

func (s *Session) syncFeatureFile(fl db.FeatureList) {
	path := filepath.Join(s.Workspace, "feature_list.json")
	data, err := json.MarshalIndent(fl, "", "  ")
	if err != nil {
		fmt.Printf("Warning: Failed to marshal feature list: %v\n", err)
		return
	}

	err = os.WriteFile(path, data, 0644)
	if err != nil && os.IsPermission(err) {
		// Try to remove it first. Host users usually have write access to the workspace directory.
		_ = os.Remove(path)
		err = os.WriteFile(path, data, 0644)
	}

	if err != nil && os.IsPermission(err) && s.ContainerID != "" {
		// Last resort: ask root-powered container to chown it back to us
		u, _ := user.Current()
		if u != nil {
			// Best effort chown
			_, _ = s.Docker.Exec(context.Background(), s.ContainerID, []string{"chown", fmt.Sprintf("%s:%s", u.Uid, u.Gid), "feature_list.json"})
			err = os.WriteFile(path, data, 0644)
		}
	}

	if err != nil {
		fmt.Printf("Warning: Failed to sync feature_list.json: %v\n", err)
	}
}

func (s *Session) checkCompletion() bool {
	return s.hasSignal("COMPLETED")
}

func (s *Session) hasSignal(name string) bool {
	if s.DBStore == nil {
		return false
	}

	// 1. Check DB first
	val, err := s.DBStore.GetSignal(name)
	if err == nil && val != "" {
		return true
	}

	// 2. Check File (Agent might have created it)
	path := filepath.Join(s.Workspace, name)
	if _, err := os.Stat(path); err == nil {
		// File exists - Migrate to DB
		fmt.Printf("Migrating signal %s from file to DB\n", name)
		if err := s.DBStore.SetSignal(name, "true"); err != nil {
			fmt.Printf("Warning: Failed to migrate signal %s to DB: %v\n", name, err)
		}
		// Remove file
		if err := os.Remove(path); err != nil {
			fmt.Printf("Warning: Failed to remove signal file %s: %v\n", name, err)
		}
		return true
	}

	return false
}

func (s *Session) clearSignal(name string) {
	if s.DBStore != nil {
		s.DBStore.DeleteSignal(name)
	}
	// Also ensure file is removed (redundancy)
	path := filepath.Join(s.Workspace, name)
	os.Remove(path)
}

// createSignal creates a signal in the DB.
func (s *Session) createSignal(name string) error {
	if s.DBStore == nil {
		return fmt.Errorf("db store not initialized")
	}
	fmt.Printf("Created signal: %s\n", name)
	return s.DBStore.SetSignal(name, "true")
}

// runQAAgent runs quality assurance checks on the feature list.
// Returns error if QA fails, nil if QA passes.
func (s *Session) runQAAgent(ctx context.Context) error {
	fmt.Println("=== QA Agent: Running Quality Checks ===")

	var qaAgent agent.Agent
	if s.QAAgent != nil {
		qaAgent = s.QAAgent
	} else {
		var err error
		qaAgent, err = agent.NewAgent("gemini-cli", os.Getenv("GEMINI_API_KEY"), "gemini-1.5-flash-latest", s.Workspace)
		if err != nil {
			return fmt.Errorf("failed to create QA agent: %w", err)
		}
	}

	// 1. Get Prompt
	prompt, err := prompts.GetPrompt(prompts.QAAgent, nil)
	if err != nil {
		return fmt.Errorf("failed to load QA prompt: %w", err)
	}

	// 2. Send to Agent
	fmt.Println("Sending verification instructions to QA Agent...")
	response, err := qaAgent.Send(ctx, prompt) // Use qaAgent
	if err != nil {
		return fmt.Errorf("QA Agent failed to respond: %w", err)
	}
	fmt.Printf("QA Agent Response (%d chars).\n", len(response))

	// 2.5 Execute Commands
	if _, err := s.ProcessResponse(ctx, response); err != nil {
		fmt.Printf("Warning: QA Agent command execution failed: %v\n", err)
	}

	// 3. Check Result File (.qa_result)
	qaResultPath := filepath.Join(s.Workspace, ".qa_result")
	data, err := os.ReadFile(qaResultPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("QA Agent did not produce .qa_result file")
		}
		return fmt.Errorf("failed to read .qa_result: %w", err)
	}
	defer os.Remove(qaResultPath) // Cleanup

	result := strings.TrimSpace(string(data))
	fmt.Printf("QA Result: %s\n", result)

	if result == "PASS" {
		if err := s.createSignal("QA_PASSED"); err != nil {
			fmt.Printf("Warning: Failed to create QA_PASSED signal: %v\n", err)
		}
		fmt.Println("QA PASSED: Agent verified all requirements.")
		return nil
	}

	fmt.Println("QA FAILED: Agent reported failure.")
	return fmt.Errorf("QA failed with result: %s", result)
}

// runManagerAgent runs manager review of the QA report.
// Returns error if manager rejects, nil if manager approves.
func (s *Session) runManagerAgent(ctx context.Context) error {
	fmt.Println("=== Manager Agent: Reviewing QA Report ===")

	var managerAgent agent.Agent
	if s.ManagerAgent != nil {
		managerAgent = s.ManagerAgent
	} else {
		var err error
		managerAgent, err = agent.NewAgent("gemini-cli", os.Getenv("GEMINI_API_KEY"), "gemini-1.5-pro-latest", s.Workspace)
		if err != nil {
			return fmt.Errorf("failed to create manager agent: %w", err)
		}
	}

	features := s.loadFeatures()
	qaReport := RunQA(features)

	// Create manager review prompt
	prompt, err := prompts.GetPrompt(prompts.ManagerReview, map[string]string{
		"qa_report": qaReport.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to load manager review prompt: %w", err)
	}

	// Send to agent for review
	fmt.Println("Sending QA report to Manager agent for review...")
	response, err := managerAgent.Send(ctx, prompt) // Use managerAgent
	if err != nil {
		return fmt.Errorf("manager review request failed: %w", err)
	}

	fmt.Printf("Manager review response (%d chars).\n", len(response))

	// Execute commands (e.g., creating PROJECT_SIGNED_OFF or deleting COMPLETED)
	if _, err := s.ProcessResponse(ctx, response); err != nil {
		fmt.Printf("Warning: Manager agent command execution failed: %v\n", err)
	}

	// Check for PROJECT_SIGNED_OFF signal
	if s.hasSignal("PROJECT_SIGNED_OFF") {
		fmt.Println("Manager APPROVED: project signed off via signal.")
		return nil
	}

	// Fallback to legacy ratio check if no explicit signal was given
	if qaReport.CompletionRatio >= 1.0 {
		fmt.Println("Manager APPROVED (Legacy/Fallback): All features passing.")
		return nil
	}

	// Manager rejected or didn't explicitly sign off
	fmt.Println("Manager REJECTED or pending: Project not signed off.")
	s.clearSignal("QA_PASSED")
	s.clearSignal("COMPLETED")
	return fmt.Errorf("manager review did not result in sign-off (ratio: %.2f)", qaReport.CompletionRatio)
}

// ProcessResponse parses the agent response for commands, executes them, and handles blockers.
func (s *Session) ProcessResponse(ctx context.Context, response string) (string, error) {
	// 1. Extract Bash Blocks
	re := regexp.MustCompile("(?s)```bash\\n(.*?)\\n```")
	matches := re.FindAllStringSubmatch(response, -1)

	// 1. Extract Bash Blocks

	var parsedOutput strings.Builder

	for _, match := range matches {
		cmdScript := match[1]
		fmt.Printf("Executing command block:\n%s\n", cmdScript)

		// Execute via Docker
		output, err := s.Docker.Exec(ctx, s.ContainerID, []string{"/bin/sh", "-c", cmdScript})
		if err != nil {
			result := fmt.Sprintf("Command Failed: %s\nError: %v\n", cmdScript, err)
			fmt.Print(result)
			parsedOutput.WriteString(result)
		} else {
			result := fmt.Sprintf("Command Output:\n%s\n", output)
			if len(output) > 0 {
				fmt.Print(result)
			}
			parsedOutput.WriteString(result)
		}

	}

	// Check for Blocker Signal (DB)
	if s.DBStore != nil {
		blockerMsg, err := s.DBStore.GetSignal("BLOCKER")
		if err == nil && blockerMsg != "" {
			fmt.Println("\n=== HUMAN INTERVENTION REQUIRED ===")
			fmt.Printf("Agent reported blocker: %s\n", blockerMsg)

			// Special handling for UI Verification
			if strings.Contains(strings.ToLower(blockerMsg), "ui verification") {
				uiData, err := os.ReadFile("ui_verification.json")
				if err == nil {
					fmt.Println("\nPending UI Verification Requests:")
					fmt.Println(string(uiData))
					fmt.Println("\nTo verify a feature, run:")
					fmt.Println("  agent-bridge verify <feature_id> <pass/fail>")
				}
			}

			fmt.Println("Session stopping to allow human resolution.")

			// Clear blocker so it doesn't loop forever? Or keep it?
			// If we keep it, restart will block again.
			// User needs to clear it or we need a mechanism.
			// For now, valid strategy is to exit.
			return "", ErrBlocker
		}
	}

	// Legacy File Check (Deprecating, but keeping for compatibility)
	if s.Docker != nil {
		blockerFiles := []string{"recac_blockers.txt", "blockers.txt"}
		for _, bf := range blockerFiles {
			checkCmd := []string{"/bin/sh", "-c", fmt.Sprintf("test -f %s && cat %s", bf, bf)}
			blockerContent, err := s.Docker.Exec(ctx, s.ContainerID, checkCmd)
			trimmed := strings.TrimSpace(blockerContent)
			if err == nil && len(trimmed) > 0 {
				// Check for false positives (status messages instead of blockers)
				// 1. Normalize: lowercase and remove common comment/bullet chars (#, *, -, whitespace)
				cleanStr := strings.ToLower(trimmed)
				cleanStr = strings.ReplaceAll(cleanStr, "#", "")
				cleanStr = strings.ReplaceAll(cleanStr, "*", "")
				cleanStr = strings.ReplaceAll(cleanStr, "-", "")
				cleanStr = strings.Join(strings.Fields(cleanStr), " ") // Normalize internal whitespace

				isFalsePositive := strings.Contains(cleanStr, "no blockers") ||
					strings.HasPrefix(cleanStr, "none") ||
					strings.Contains(cleanStr, "no technical obstacles") ||
					strings.Contains(cleanStr, "progressing smoothly") ||
					strings.Contains(cleanStr, "initial setup complete") ||
					strings.Contains(cleanStr, "all requirements met") ||
					strings.Contains(cleanStr, "ready for next feature")

				if isFalsePositive {
					fmt.Printf("Ignoring false positive blocker in %s: %s\n", bf, trimmed)
					// Cleanup the file so it doesn't re-trigger
					s.Docker.Exec(ctx, s.ContainerID, []string{"rm", bf})
					continue
				}

				// Real Blocker found!
				fmt.Printf("\n=== HUMAN INTERVENTION REQUIRED (File: %s) ===\n", bf)
				fmt.Printf("Agent reported blocker:\n%s\n", blockerContent)

				// Special handling for UI Verification
				if strings.Contains(strings.ToLower(blockerContent), "ui verification") {
					uiData, err := os.ReadFile("ui_verification.json")
					if err == nil {
						fmt.Println("\nPending UI Verification Requests:")
						fmt.Println(string(uiData))
						fmt.Println("\nTo verify a feature, run:")
						fmt.Println("  agent-bridge verify <feature_id> <pass/fail>")
					}
				}

				fmt.Println("Session stopping to allow human resolution.")
				return "", ErrBlocker
			}
		}
	}

	return parsedOutput.String(), nil
}

// runCleanerAgent removes temporary files listed in temp_files.txt.
func (s *Session) runCleanerAgent(ctx context.Context) error {
	fmt.Println("=== Cleaner Agent: Removing Temporary Files ===")

	// Check if temp_files.txt exists
	tempFilesPath := filepath.Join(s.Workspace, "temp_files.txt")
	if _, err := os.Stat(tempFilesPath); os.IsNotExist(err) {
		fmt.Println("No temp_files.txt found. Nothing to clean.")
		return nil // Nothing to clean
	}

	data, err := os.ReadFile(tempFilesPath)
	if err != nil {
		return fmt.Errorf("failed to read temp_files.txt: %w", err)
	}

	// Parse temp files (one per line)
	lines := strings.Split(string(data), "\n")
	cleaned := 0
	errors := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip empty lines and comments
		}

		// Handle both relative and absolute paths
		var filePath string
		if filepath.IsAbs(line) {
			filePath = line
		} else {
			filePath = filepath.Join(s.Workspace, line)
		}

		if err := os.Remove(filePath); err != nil {
			if !os.IsNotExist(err) {
				fmt.Printf("Warning: Failed to remove %s: %v\n", line, err)
				errors++
			}
		} else {
			fmt.Printf("Removed: %s\n", line)
			cleaned++
		}
	}

	fmt.Printf("Cleaner complete: %d files removed, %d errors\n", cleaned, errors)

	// Clear the temp_files.txt itself
	os.Remove(tempFilesPath)

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// injectAgentBridge copies the agent-bridge binary to the workspace
func (s *Session) injectAgentBridge() error {
	// Find agent-bridge binary
	// 1. Try CWD
	srcPath, err := filepath.Abs("agent-bridge")
	if err != nil {
		return err
	}

	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		// 2. Try Project Root (assuming we are in internal/runner or a sub-test dir)
		// Go up 2 levels: internal/runner -> root
		// Or 3 levels if inside a test package
		// Let's try finding go.mod
		dir, _ := os.Getwd()
		for i := 0; i < 5; i++ { // Guard against infinite loop
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				// Found root
				srcPath = filepath.Join(dir, "agent-bridge")
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// Destination: workspace/agent-bridge
	destPath := filepath.Join(s.Workspace, "agent-bridge")

	// Copy file
	input, err := os.ReadFile(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("agent-bridge binary not found at %s. Did you run 'make bridge'?", srcPath)
		}
		return err
	}

	if err := os.WriteFile(destPath, input, 0755); err != nil {
		return err
	}

	fmt.Printf("Injected agent-bridge to %s\n", destPath)
	return nil
}

// cleanupAgentBridge removes the agent-bridge binary from the workspace
func (s *Session) cleanupAgentBridge() error {
	path := filepath.Join(s.Workspace, "agent-bridge")
	return os.Remove(path)
}

// checkAutoQA checks if all features pass and we haven't already passed QA/Completed
func (s *Session) checkAutoQA() bool {
	if s.hasSignal("QA_PASSED") || s.hasSignal("COMPLETED") || s.hasSignal("PROJECT_SIGNED_OFF") {
		return false
	}

	features := s.loadFeatures()
	if len(features) == 0 {
		return false
	}

	allPass := true
	for _, f := range features {
		if !(f.Passes || f.Status == "done" || f.Status == "implemented") {
			allPass = false
			break
		}
	}

	if allPass {
		if err := s.createSignal("COMPLETED"); err != nil {
			fmt.Printf("Warning: Failed to create COMPLETED signal: %v\n", err)
		}
		return true
	}

	return false
}
