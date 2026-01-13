package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"recac/internal/agent"
	"recac/internal/docker"
	"recac/internal/git"
	"recac/internal/jira"
	"recac/internal/runner"
	"recac/internal/telemetry"
	"recac/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	startCmd.Flags().String("path", "", "Project path (skips wizard)")
	startCmd.Flags().Int("max-iterations", 30, "Maximum number of iterations")
	startCmd.Flags().Int("manager-frequency", 5, "Frequency of manager reviews")
	startCmd.Flags().Int("max-agents", 1, "Maximum number of parallel agents")
	startCmd.Flags().Int("task-max-iterations", 10, "Maximum iterations for sub-tasks")
	startCmd.Flags().Bool("detached", false, "Run session in background (detached mode)")
	startCmd.Flags().String("name", "", "Name for the session (required for detached mode)")
	startCmd.Flags().String("jira", "", "Jira Ticket ID to start session from (e.g. PROJ-123)")
	startCmd.Flags().Bool("manager-first", false, "Run the Manager Agent before the first coding session")
	startCmd.Flags().Bool("stream", false, "Stream agent output to the console")
	startCmd.Flags().Bool("allow-dirty", false, "Allow running with uncommitted git changes")
	viper.BindPFlag("path", startCmd.Flags().Lookup("path"))
	viper.BindPFlag("max_iterations", startCmd.Flags().Lookup("max-iterations"))
	viper.BindPFlag("manager_frequency", startCmd.Flags().Lookup("manager-frequency"))
	viper.BindPFlag("max_agents", startCmd.Flags().Lookup("max-agents"))
	viper.BindPFlag("task_max_iterations", startCmd.Flags().Lookup("task-max-iterations"))
	viper.BindPFlag("detached", startCmd.Flags().Lookup("detached"))
	viper.BindPFlag("name", startCmd.Flags().Lookup("name"))
	viper.BindPFlag("jira", startCmd.Flags().Lookup("jira"))
	viper.BindPFlag("manager_first", startCmd.Flags().Lookup("manager-first"))
	viper.BindPFlag("stream", startCmd.Flags().Lookup("stream"))
	viper.BindPFlag("allow_dirty", startCmd.Flags().Lookup("allow-dirty"))
	startCmd.Flags().String("jira-label", "", "Jira Label to find tickets (e.g. agent-work)")
	startCmd.Flags().Int("max-parallel-tickets", 1, "Maximum number of Jira tickets to process in parallel")
	viper.BindPFlag("jira_label", startCmd.Flags().Lookup("jira-label"))
	viper.BindPFlag("max_parallel_tickets", startCmd.Flags().Lookup("max-parallel-tickets"))
	startCmd.Flags().Bool("auto-merge", false, "Automatically merge PRs if checks pass")
	viper.BindPFlag("auto_merge", startCmd.Flags().Lookup("auto-merge"))
	startCmd.Flags().Bool("skip-qa", false, "Skip QA phase and auto-complete (use with caution)")
	viper.BindPFlag("skip_qa", startCmd.Flags().Lookup("skip-qa"))
	startCmd.Flags().String("image", "ghcr.io/process-failed-successfully/recac-agent:latest", "Docker image to use for the agent session")
	viper.BindPFlag("image", startCmd.Flags().Lookup("image"))
	startCmd.Flags().Bool("cleanup", true, "Cleanup temporary workspace after session ends")
	viper.BindPFlag("cleanup", startCmd.Flags().Lookup("cleanup"))
	startCmd.Flags().String("project", "", "Project name override")
	viper.BindPFlag("project", startCmd.Flags().Lookup("project"))

	// Internal flag for resuming sessions
	if startCmd.Flags().Lookup("resume-from") == nil {
		startCmd.Flags().String("resume-from", "", "Resume from a specific workspace path")
		startCmd.Flags().MarkHidden("resume-from")
	}

	startCmd.Flags().String("repo-url", "", "Repository URL to clone (bypasses Jira if provided)")
	startCmd.Flags().String("summary", "", "Task summary (bypasses Jira if provided)")
	startCmd.Flags().String("description", "", "Task description")
	viper.BindPFlag("repo_url", startCmd.Flags().Lookup("repo-url"))
	viper.BindPFlag("summary", startCmd.Flags().Lookup("summary"))
	viper.BindPFlag("description", startCmd.Flags().Lookup("description"))

	viper.BindEnv("max_iterations", "RECAC_MAX_ITERATIONS")
	viper.BindEnv("manager_frequency", "RECAC_MANAGER_FREQUENCY")
	viper.BindEnv("task_max_iterations", "RECAC_TASK_MAX_ITERATIONS")

	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start an autonomous coding session",
	Long:  `Start the agent execution loop to perform coding tasks autonomously.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Panic recovery for graceful shutdown
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "\n=== CRITICAL ERROR: Session Panic ===\n")
				fmt.Fprintf(os.Stderr, "Error: %v\n", r)
				fmt.Fprintf(os.Stderr, "Attempting graceful shutdown...\n")
				exit(1)
			}
		}()

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		debug := viper.GetBool("debug")
		isMock, _ := cmd.Flags().GetBool("mock")
		if !isMock {
			isMock = viper.GetBool("mock")
		}
		projectPath := viper.GetString("path")
		if pathFlag, _ := cmd.Flags().GetString("path"); pathFlag != "" {
			projectPath = pathFlag
		}

		provider, _ := cmd.Flags().GetString("provider")
		if provider == "" {
			provider = viper.GetString("provider")
		}
		model, _ := cmd.Flags().GetString("model")
		if model == "" {
			model = viper.GetString("model")
		}

		projectName := viper.GetString("project")
		if projectFlag, _ := cmd.Flags().GetString("project"); projectFlag != "" {
			projectName = projectFlag
		}

		maxIterations := viper.GetInt("max_iterations")
		if maxIterFlag, _ := cmd.Flags().GetInt("max-iterations"); cmd.Flags().Changed("max-iterations") {
			maxIterations = maxIterFlag
		}
		managerFrequency := viper.GetInt("manager_frequency")
		maxAgents := viper.GetInt("max_agents")
		taskMaxIterations := viper.GetInt("task_max_iterations")
		detached := viper.GetBool("detached")
		sessionName := viper.GetString("name")

		jiraTicketID, _ := cmd.Flags().GetString("jira")
		if jiraTicketID == "" {
			jiraTicketID = viper.GetString("jira")
		}

		// Handle Jira Ticket Workflow
		jiraLabel := viper.GetString("jira_label")

		// Persistent Flags used in config
		autoMergeFlag, _ := cmd.Flags().GetBool("auto-merge")
		skipQAFlag, _ := cmd.Flags().GetBool("skip-qa")

		repoURL, _ := cmd.Flags().GetString("repo-url")
		summary, _ := cmd.Flags().GetString("summary")
		description, _ := cmd.Flags().GetString("description")

		// Global Configuration
		cfg := SessionConfig{
			ProjectPath:       projectPath,
			IsMock:            isMock,
			MaxIterations:     maxIterations,
			ManagerFrequency:  managerFrequency,
			MaxAgents:         maxAgents,
			TaskMaxIterations: taskMaxIterations,
			Detached:          detached,
			SessionName:       sessionName,
			AllowDirty:        viper.GetBool("allow_dirty"),
			Stream:            viper.GetBool("stream"),
			AutoMerge:         autoMergeFlag || viper.GetBool("auto_merge"),
			SkipQA:            skipQAFlag || viper.GetBool("skip_qa"),
			ManagerFirst:      viper.GetBool("manager_first"),
			Image:             viper.GetString("image"),
			Debug:             debug,
			Provider:          provider,
			Model:             model,
			Cleanup:           viper.GetBool("cleanup"),
			ProjectName:       projectName,
			RepoURL:           repoURL,
			Summary:           summary,
			Description:       description,
		}

		// Handle session resumption
		if resumePath, _ := cmd.Flags().GetString("resume-from"); resumePath != "" {
			cfg.ProjectPath = resumePath
			fmt.Printf("Resuming session '%s' from workspace: %s\n", cfg.SessionName, resumePath)
			if err := runWorkflow(ctx, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Resumed session failed: %v\n", err)
				exit(1)
			}
			return
		}

		if repoURL != "" {
			processDirectTask(ctx, cfg)
			return
		}

		if jiraTicketID != "" || jiraLabel != "" {
			jClient, err := getJiraClient(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				exit(1)
			}

			// 1.5 Collect Ticket IDs
			var ticketIDs []string
			if jiraTicketID != "" {
				ticketIDs = append(ticketIDs, jiraTicketID)
			} else if jiraLabel != "" {
				fmt.Printf("Searching for tickets with label '%s'...\n", jiraLabel)
				jql := fmt.Sprintf("labels = \"%s\" AND statusCategory != Done ORDER BY created DESC", jiraLabel)
				issues, err := jClient.SearchIssues(ctx, jql)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error searching Jira tickets: %v\n", err)
					exit(1)
				}

				if len(issues) == 0 {
					fmt.Printf("No open tickets found with label '%s'. Exiting.\n", jiraLabel)
					return
				}

				// Sort issues by dependencies (blockers first)
				sortedIssues, err := jira.ResolveDependencies(issues, func(issue map[string]interface{}) ([]string, error) {
					// We need to fetch the full issue to get links if not present?
					// But our Search includes "issuelinks".
					// Does ResolveDependencies expect keys?
					// Yes, Update ResolveDependencies usage.
					// Actually, GetBlockers returns formatted strings "KEY (Status)".
					// We need just keys for dependency graph.
					// Let's make a wrapper or update GetBlockers.
					// For now, let's just extract the Key from GetBlockers output or reimplement simple key extraction here.

					rawBlockers := jClient.GetBlockers(issue)
					var keys []string
					for _, b := range rawBlockers {
						// Format is "KEY (Status)"
						parts := strings.Split(b, " (")
						if len(parts) > 0 {
							keys = append(keys, parts[0])
						}
					}
					return keys, nil
				})

				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to sort issues by dependency: %v. Proceeding with default order.\n", err)
					sortedIssues = issues
				} else {
					fmt.Println("Tickets sorted by dependency (blockers first).")
				}

				for _, issue := range sortedIssues {
					if key, ok := issue["key"].(string); ok {
						ticketIDs = append(ticketIDs, key)
					}
				}
				fmt.Printf("Found %d tickets to process.\n", len(ticketIDs))
			}

			// Parallel Multi-Ticket Processing Loop
			maxParallelTickets := viper.GetInt("max_parallel_tickets")
			if maxParallelTickets < 1 {
				maxParallelTickets = 1
			}

			// If only one ticket, run synchronously
			if len(ticketIDs) == 1 {
				processJiraTicket(ctx, ticketIDs[0], jClient, cfg, nil)
				return
			}

			// Build Dependency Graphs
			// Fetch metadata for all tickets to ensure we have dependency info
			// (Even if we came from Search, we need to be sure we have the objects)
			fmt.Println("Building dependency graph for parallel execution...")
			issues := make([]map[string]interface{}, 0, len(ticketIDs))
			for _, id := range ticketIDs {
				t, err := jClient.GetTicket(ctx, id)
				if err != nil {
					fmt.Printf("Warning: Failed to fetch metadata for %s: %v. Assuming no dependencies.\n", id, err)
					issues = append(issues, map[string]interface{}{"key": id})
				} else {
					issues = append(issues, t)
				}
			}

			graph := jira.BuildGraphFromIssues(issues, func(issue map[string]interface{}) []string {
				raw := jClient.GetBlockers(issue)
				keys := make([]string, 0, len(raw))
				for _, r := range raw {
					// Format "KEY (Status)"
					parts := strings.Split(r, " (")
					if len(parts) > 0 {
						keys = append(keys, parts[0])
					}
				}
				return keys
			})

			// Channels
			readyCh := make(chan string, len(ticketIDs))
			completionCh := make(chan string, len(ticketIDs))

			// Coordinator Routine
			go func() {
				completed := make(map[string]bool)
				dispatched := make(map[string]bool)
				count := 0
				total := len(ticketIDs)

				// Initial Dispatch
				initial := graph.GetReadyTickets(completed)
				for _, id := range initial {
					readyCh <- id
					dispatched[id] = true
				}

				for count < total {
					doneID := <-completionCh
					count++
					completed[doneID] = true
					// fmt.Printf("Coordinator: %s finished. (%d/%d)\n", doneID, count, total)

					// Check for new ready tickets
					candidates := graph.GetReadyTickets(completed)
					for _, id := range candidates {
						if !dispatched[id] {
							readyCh <- id
							dispatched[id] = true
						}
					}
				}
				close(readyCh)
			}()

			// Worker Loop
			var wg sync.WaitGroup
			sem := make(chan struct{}, maxParallelTickets)

			for id := range readyCh {
				wg.Add(1)
				go func(targetID string) {
					defer wg.Done()

					// Acquire Semaphore
					sem <- struct{}{}
					defer func() { <-sem }()

					localTickets := graph.AllTickets
					processJiraTicket(ctx, targetID, jClient, cfg, localTickets)

					// Signal Completion
					completionCh <- targetID
				}(id)
			}

			wg.Wait()
			return
		}

		// Local Path Workflow
		if cfg.ProjectPath == "" {
			p := tea.NewProgram(ui.NewWizardModel())
			m, err := p.Run()
			if err != nil {
				fmt.Printf("Wizard error: %v", err)
				exit(1)
			}

			wizardModel, ok := m.(ui.WizardModel)
			if !ok {
				fmt.Println("Could not retrieve wizard data")
				exit(1)
			}
			cfg.ProjectPath = wizardModel.Path
			if cfg.ProjectPath == "" {
				fmt.Println("No project path selected. Exiting.")
				return
			}

			if wizardModel.Provider != "" {
				viper.Set("provider", wizardModel.Provider)
			}
			if wizardModel.MaxAgents > 0 {
				cfg.MaxAgents = wizardModel.MaxAgents
			}
			if wizardModel.TaskMaxIterations > 0 {
				cfg.TaskMaxIterations = wizardModel.TaskMaxIterations
			}
		} else {
			fmt.Printf("Using project path: %s\n", cfg.ProjectPath)
		}

		if err := runWorkflow(ctx, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Session failed: %v\n", err)
			exit(1)
		}
	},
}

// SessionConfig holds all parameters for a RECAC session
type SessionConfig struct {
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
}

// processDirectTask handles a coding session from a direct repository and task description
func processDirectTask(ctx context.Context, cfg SessionConfig) {
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
			return
		}
	}

	if _, err := setupWorkspace(ctx, cfg.RepoURL, cfg.ProjectPath, workID, "", timestamp); err != nil {
		logger.Error("Error: Failed to setup workspace", "error", err)
		return
	}

	// Force task context: Overwrite app_spec.txt and remove feature_list.json
	if cfg.Summary != "" || cfg.Description != "" {
		specContent := fmt.Sprintf("# Task Summary: %s\n\n%s", cfg.Summary, cfg.Description)
		specPath := filepath.Join(cfg.ProjectPath, "app_spec.txt")
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			logger.Error("Error writing app_spec.txt", "error", err)
			return
		}

		logger.Info("Refreshed workspace context from task description")
	}

	// Update configuration for the session run
	// cfg.ProjectPath is already set correctly above

	// Run Workflow
	if err := runWorkflow(ctx, cfg); err != nil {
		logger.Error("Session failed", "error", err)
	} else {
		logger.Info("Session completed successfully")
	}
}

// processJiraTicket handles the Jira-specific workflow and then runs the project session
func processJiraTicket(ctx context.Context, jiraTicketID string, jClient *jira.Client, cfg SessionConfig, ignoredBlockers map[string]bool) {
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
		return
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
			return
		}
	}

	// Extract details
	fields, ok := ticket["fields"].(map[string]interface{})
	if !ok {
		logger.Error("Error: Invalid ticket format (missing fields)")
		return
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
			exit(1)
		}
		logger.Info("Using provided workspace path", "path", tempWorkspace)
	} else {
		pattern := fmt.Sprintf("recac-jira-%s-%s-*", jiraTicketID, timestamp)
		tempWorkspace, err = os.MkdirTemp("", pattern)
		if err != nil {
			logger.Error("Error creating temp workspace", "error", err)
			return
		}
	}

	repoRegex := regexp.MustCompile(`(?i)Repo: (https?://\S+)`)
	matches := repoRegex.FindStringSubmatch(description)
	if len(matches) <= 1 {
		logger.Error("Error: No repository URL found in ticket description (Repo: https://...)")
		exit(1)
	}

	repoURL := strings.TrimSuffix(matches[1], ".git")
	logger.Info("Found repository URL in ticket", "repo_url", repoURL)

	if _, err := setupWorkspace(ctx, repoURL, tempWorkspace, jiraTicketID, cfg.JiraEpicKey, timestamp); err != nil {
		logger.Error("Error: Failed to setup workspace", "error", err)
		exit(1)
	}

	// 5. Create app_spec.txt
	specContent := fmt.Sprintf("# Jira Ticket: %s\n# Summary: %s\n\n%s", jiraTicketID, summary, description)
	specPath := filepath.Join(tempWorkspace, "app_spec.txt")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		logger.Error("Error writing app_spec.txt", "error", err)
		return
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
	if err := runWorkflow(ctx, cfg); err != nil {
		logger.Error("Session failed", "error", err)
	} else {
		logger.Info("Session completed successfully")
	}
}

// runWorkflow handles the execution of a single project session (local or Jira-based)
func runWorkflow(ctx context.Context, cfg SessionConfig) error {
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

		command := []string{executable, "start"}
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

		sm, err := runner.NewSessionManager()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %v", err)
		}

		// Get the starting commit SHA
		var startSHA string
		gitClient := git.NewClient()
		sha, err := gitClient.CurrentCommitSHA(projectPath)
		if err != nil {
			fmt.Printf("Warning: could not get start commit SHA: %v\n", err)
		} else {
			startSHA = sha
		}

		session, err := sm.StartSession(cfg.SessionName, command, projectPath)
		if err != nil {
			return fmt.Errorf("failed to start detached session: %v", err)
		}

		// Save the start commit SHA to the session state
		if startSHA != "" {
			session.StartCommitSHA = startSHA
			if err := sm.SaveSession(session); err != nil {
				fmt.Printf("Warning: failed to save start commit SHA for session %s: %v\n", cfg.SessionName, err)
			}
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

		session := runner.NewSession(dockerCli, agentClient, projectPath, cfg.Image, projectName, cfg.Provider, cfg.Model, cfg.MaxAgents)
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
	agentClient, err := getAgentClient(ctx, provider, model, projectPath, projectName)
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %v", err)
	}

	session := runner.NewSession(dockerCli, agentClient, projectPath, cfg.Image, projectName, provider, model, cfg.MaxAgents)
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

	// Get the starting commit SHA for interactive sessions
	var startSHA string
	gitClient := git.NewClient()
	if sha, err := gitClient.CurrentCommitSHA(projectPath); err == nil {
		startSHA = sha
	} else {
		fmt.Printf("Warning: could not get start commit SHA: %v\n", err)
	}

	if err := session.Start(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}

	// Create a session state for the interactive session to track commit SHAs
	sm, err := runner.NewSessionManager()
	if err != nil {
		return fmt.Errorf("failed to create session manager for interactive session: %w", err)
	}
	interactiveSessionState := &runner.SessionState{
		Name:           cfg.SessionName,
		StartTime:      time.Now(),
		Command:        os.Args,
		Workspace:      projectPath,
		Status:         "running",
		Type:           "interactive",
		AgentStateFile: filepath.Join(projectPath, ".agent_state.json"),
		StartCommitSHA: startSHA,
	}
	sm.SaveSession(interactiveSessionState)

	runErr := session.RunLoop(ctx)

	// Now that the session is over, get the end commit SHA
	endSHA, err := gitClient.CurrentCommitSHA(projectPath)
	if err != nil {
		fmt.Printf("Warning: could not get end commit SHA: %v\n", err)
	}

	// Update the session state
	interactiveSessionState.EndCommitSHA = endSHA
	interactiveSessionState.EndTime = time.Now()
	if runErr != nil {
		interactiveSessionState.Status = "error"
		interactiveSessionState.Error = runErr.Error()
	} else {
		interactiveSessionState.Status = "completed"
	}
	sm.SaveSession(interactiveSessionState)

	return runErr
}
