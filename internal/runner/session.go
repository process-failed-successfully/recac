package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/agent/prompts"
	"recac/internal/db"
	"recac/internal/docker"
	"recac/internal/security"
	"strings"
	"time"
)

type Session struct {
	Docker           *docker.Client
	Agent            agent.Agent
	Workspace        string
	Image            string
	SpecFile         string
	Iteration        int
	MaxIterations    int
	ManagerFrequency int
	AgentStateFile   string              // Path to agent state file (.agent_state.json)
	StateManager     *agent.StateManager // State manager for agent state persistence
	DBStore          db.Store            // Persistent database store
	Scanner          security.Scanner    // Security scanner
	ContainerID      string              // Container ID for cleanup
}

func NewSession(d *docker.Client, a agent.Agent, workspace, image string) *Session {
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
func NewSessionWithStateFile(d *docker.Client, a agent.Agent, workspace, image, agentStateFile string) *Session {
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

	// Run Container
	id, err := s.Docker.RunContainer(ctx, s.Image, s.Workspace, extraBinds)
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	s.ContainerID = id
	fmt.Printf("Container started successfully. ID: %s\n", id)
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
		response, err := s.Agent.Send(ctx, prompt)
		if err != nil {
			fmt.Printf("Agent error: %v. Retrying in next iteration...\n", err)
			time.Sleep(2 * time.Second)
			continue
		}

		fmt.Printf("Response received (%d chars).\n", len(response))

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

		// Save agent state periodically (every iteration)
		if err := s.SaveAgentState(); err != nil {
			fmt.Printf("Warning: Failed to save agent state: %v\n", err)
		}

		// Handle Lifecycle Role Transitions (Agent-QA-Manager-Cleaner workflow)
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
				// Manager rejected - clear QA_PASSED and continue coding
				s.clearSignal("QA_PASSED")
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
	if s.Iteration == 1 {
		spec, _ := s.ReadSpec()
		prompt, err := prompts.GetPrompt(prompts.Initializer, map[string]string{
			"spec": spec,
		})
		return prompt, false, err
	}

	// 2. Manager Review (Triggered by file or frequency)
	if s.Iteration%s.ManagerFrequency == 0 || s.hasSignal("TRIGGER_MANAGER") {
		// Cleanup signal
		s.clearSignal("TRIGGER_MANAGER")

		features := s.loadFeatures()
		qaReport := RunQA(features) // Mock features for now as we don't track them in Session yet
		prompt, err := prompts.GetPrompt(prompts.ManagerReview, map[string]string{
			"qa_report": qaReport.String(),
		})
		return prompt, true, err
	}

	// 3. Coding Agent (Default)
	prompt, err := prompts.GetPrompt(prompts.CodingAgent, nil)
	return prompt, false, err
}

func (s *Session) loadFeatures() []Feature {
	path := filepath.Join(s.Workspace, "feature_list.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var features []Feature
	if err := json.Unmarshal(data, &features); err != nil {
		return nil
	}
	return features
}

func (s *Session) checkCompletion() bool {
	return s.hasSignal("COMPLETED")
}

func (s *Session) hasSignal(name string) bool {
	path := filepath.Join(s.Workspace, name)
	_, err := os.Stat(path)
	return err == nil
}

func (s *Session) clearSignal(name string) {
	path := filepath.Join(s.Workspace, name)
	os.Remove(path)
}

// createSignal creates a sentinel file in the workspace.
func (s *Session) createSignal(name string) error {
	path := filepath.Join(s.Workspace, name)
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create signal file %s: %w", name, err)
	}
	file.Close()
	fmt.Printf("Created sentinel file: %s\n", name)
	return nil
}

// runQAAgent runs quality assurance checks on the feature list.
// Returns error if QA fails, nil if QA passes.
func (s *Session) runQAAgent(ctx context.Context) error {
	fmt.Println("=== QA Agent: Running Quality Checks ===")

	features := s.loadFeatures()
	if len(features) == 0 {
		return fmt.Errorf("no features found in feature_list.json")
	}

	qaReport := RunQA(features)
	fmt.Println(qaReport.String())

	// QA passes if all features pass
	if qaReport.FailedFeatures > 0 {
		fmt.Printf("QA FAILED: %d features still failing\n", qaReport.FailedFeatures)
		return fmt.Errorf("QA checks failed: %d/%d features passing", qaReport.PassedFeatures, qaReport.TotalFeatures)
	}

	fmt.Println("QA PASSED: All features are passing")
	return nil
}

// runManagerAgent runs manager review of the QA report.
// Returns error if manager rejects, nil if manager approves.
func (s *Session) runManagerAgent(ctx context.Context) error {
	fmt.Println("=== Manager Agent: Reviewing QA Report ===")

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
	response, err := s.Agent.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("manager review request failed: %w", err)
	}

	fmt.Printf("Manager review response (%d chars): %s\n", len(response), response[:min(200, len(response))])

	// For now, manager approves if QA report shows 100% completion
	// In a full implementation, the agent response would be parsed to determine approval
	if qaReport.CompletionRatio >= 1.0 {
		fmt.Println("Manager APPROVED: All features passing, project ready for sign-off")
		return nil
	}

	// Manager may still approve even with some failures (agent decision)
	// For this implementation, we'll approve if completion ratio is high enough
	if qaReport.CompletionRatio >= 0.95 {
		fmt.Println("Manager APPROVED: High completion ratio, project ready for sign-off")
		return nil
	}

	fmt.Println("Manager REJECTED: Completion ratio too low")
	return fmt.Errorf("manager rejected: only %.1f%% completion", qaReport.CompletionRatio*100)
}

// runCleanerAgent removes temporary files listed in temp_files.txt.
func (s *Session) runCleanerAgent(ctx context.Context) error {
	fmt.Println("=== Cleaner Agent: Removing Temporary Files ===")

	tempFilesPath := filepath.Join(s.Workspace, "temp_files.txt")
	data, err := os.ReadFile(tempFilesPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No temp_files.txt found. Nothing to clean.")
			return nil
		}
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
