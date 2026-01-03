package main

import (
	"context"
	"fmt"
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
	"recac/internal/jira"
	"recac/internal/runner"
	"recac/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	startCmd.Flags().Bool("mock", false, "Start in mock mode (no Docker or API keys required)")
	startCmd.Flags().String("path", "", "Project path (skips wizard)")
	startCmd.Flags().Int("max-iterations", 20, "Maximum number of iterations")
	startCmd.Flags().Int("manager-frequency", 5, "Frequency of manager reviews")
	startCmd.Flags().Int("max-agents", 1, "Maximum number of parallel agents")
	startCmd.Flags().Int("task-max-iterations", 10, "Maximum iterations for sub-tasks")
	startCmd.Flags().Bool("detached", false, "Run session in background (detached mode)")
	startCmd.Flags().String("name", "", "Name for the session (required for detached mode)")
	startCmd.Flags().String("jira", "", "Jira Ticket ID to start session from (e.g. PROJ-123)")
	startCmd.Flags().Bool("manager-first", false, "Run the Manager Agent before the first coding session")
	startCmd.Flags().Bool("stream", false, "Stream agent output to the console")
	startCmd.Flags().Bool("allow-dirty", false, "Allow running with uncommitted git changes")
	viper.BindPFlag("mock", startCmd.Flags().Lookup("mock"))
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
	startCmd.Flags().String("model", "", "Model to use (overrides config and RECAC_MODEL env var)")
	startCmd.Flags().String("provider", "", "Agent provider (gemini, gemini-cli, openai, etc)")
	viper.BindPFlag("model", startCmd.Flags().Lookup("model"))
	viper.BindPFlag("provider", startCmd.Flags().Lookup("provider"))
	startCmd.Flags().String("jira-label", "", "Jira Label to find tickets (e.g. agent-work)")
	startCmd.Flags().Int("max-parallel-tickets", 1, "Maximum number of Jira tickets to process in parallel")
	viper.BindPFlag("jira_label", startCmd.Flags().Lookup("jira-label"))
	viper.BindPFlag("max_parallel_tickets", startCmd.Flags().Lookup("max-parallel-tickets"))
	startCmd.Flags().Bool("auto-merge", false, "Automatically merge PRs if checks pass")
	viper.BindPFlag("auto_merge", startCmd.Flags().Lookup("auto-merge"))
	startCmd.Flags().Bool("skip-qa", false, "Skip QA phase and auto-complete (use with caution)")
	viper.BindPFlag("skip_qa", startCmd.Flags().Lookup("skip-qa"))
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
		isMockFlag, _ := cmd.Flags().GetBool("mock")
		isMock := isMockFlag || viper.GetBool("mock")
		projectPath := viper.GetString("path")
		if pathFlag, _ := cmd.Flags().GetString("path"); pathFlag != "" {
			projectPath = pathFlag
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
		jiraTicketID := viper.GetString("jira")
		// Handle Jira Ticket Workflow
		jiraLabel := viper.GetString("jira_label")

		// Persistent Flags used in config
		autoMergeFlag, _ := cmd.Flags().GetBool("auto-merge")
		skipQAFlag, _ := cmd.Flags().GetBool("skip-qa")

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
			Debug:             debug,
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
				processJiraTicket(ctx, ticketIDs[0], jClient, cfg)
				return
			}

			// Build Dependency Graphs
			localTickets := make(map[string]bool)
			for _, id := range ticketIDs {
				localTickets[id] = true
			}

			// internalBlockers[A] = [B, C] -> A is blocked by B and C
			internalBlockers := make(map[string]map[string]bool)
			// blockedBy[B] = [A, D] -> B blocks A and D
			blockedBy := make(map[string][]string)

			// Pre-calculate dependencies
			// We iterate sortedIssues to get the full data if available, or fetch if needed.
			// Re-fetching efficient enough? We already have sortedIssues inside the if block,
			// but we are outside it now?
			// sortedIssues scope is inside 'if jiraLabel != ""'.
			// We can't access sortedIssues here if ticketIDs implies we came from there.
			// Actually ticketIDs is just strings.
			// But we have jClient.
			// Let's iterate ticketIDs and define dependencies.
			// Note: processJiraTicket calls GetTicket -> GetBlockers.
			// We should avoid double fetching if possible, but for correct ordering we need it.
			// Since we did SearchIssues previously, we have the data, but we discarded it into ticketIDs.
			// Just re-fetching blockers for dependency graph is safer than assuming sort order equals dependency map.

			// Optimization: If we came from Search, we effectively have the issues.
			// But scoping prevents access.
			// Let's just fetch blockers. It's metadata light.

			fmt.Println("Building dependency graph for parallel execution...")
			for _, id := range ticketIDs {
				// We need basic issue data to get blockers.
				// We can assume GetTicket is cached or fast enough.
				// Or use a lightweight check.
				// Actually, JClient.GetTicket makes an API call.
				// Doing this 15 times before starting is okay-ish.
				t, err := jClient.GetTicket(ctx, id)
				if err != nil {
					fmt.Printf("Warning: Failed to fetch metadata for %s: %v. Assuming no dependencies.\n", id, err)
					continue
				}

				blockers := jClient.GetBlockers(t)
				for _, bRaw := range blockers {
					// Format "KEY (Status)"
					parts := strings.Split(bRaw, " (")
					if len(parts) > 0 {
						bKey := parts[0]
						if localTickets[bKey] {
							if internalBlockers[id] == nil {
								internalBlockers[id] = make(map[string]bool)
							}
							internalBlockers[id][bKey] = true
							blockedBy[bKey] = append(blockedBy[bKey], id)
						}
					}
				}
			}

			// Channels
			readyCh := make(chan string, len(ticketIDs))
			completionCh := make(chan string, len(ticketIDs))

			// Initial Ready Set
			for _, id := range ticketIDs {
				if len(internalBlockers[id]) == 0 {
					readyCh <- id
				}
			}

			// Worker Pool
			var wg sync.WaitGroup
			// We use a semaphore for limiting CONCURRENT execution, but we use strict dependency ordering for STARTING.
			sem := make(chan struct{}, maxParallelTickets)

			// Start Coordinator Routine
			// Monitors completion and schedules dependents
			go func() {
				// We expect 'len(ticketIDs)' completions
				completed := 0
				total := len(ticketIDs)

				for completed < total {
					doneID := <-completionCh
					completed++
					// fmt.Printf("Coordinator: %s finished. (%d/%d)\n", doneID, completed, total)

					// Release dependents
					for _, depID := range blockedBy[doneID] {
						if internalBlockers[depID] != nil {
							delete(internalBlockers[depID], doneID)
							if len(internalBlockers[depID]) == 0 {
								// No more local blockers!
								readyCh <- depID
							}
						}
					}
				}
				close(readyCh)
			}()

			// Worker Loop
			// We interpret 'readyCh' as "Tickets ready to be processed respecting dependencies"
			// We respect maxParallelTickets for CPU/Agent-limit.
			for id := range readyCh {
				wg.Add(1)
				go func(targetID string) {
					defer wg.Done()

					// Acquire Semaphore
					sem <- struct{}{}
					defer func() { <-sem }()

					// Process
					processJiraTicket(ctx, targetID, jClient, cfg)

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
}

// processJiraTicket handles the Jira-specific workflow and then runs the project session
func processJiraTicket(ctx context.Context, jiraTicketID string, jClient *jira.Client, cfg SessionConfig) {
	// 2. Fetch Ticket
	ticket, err := jClient.GetTicket(ctx, jiraTicketID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s] Error fetching Jira ticket: %v\n", jiraTicketID, err)
		return
	}

	// 2a. Check for Blockers
	blockers := jClient.GetBlockers(ticket)
	if len(blockers) > 0 {
		fmt.Printf("[%s] SKIPPING: Ticket is blocked by: %s\n", jiraTicketID, strings.Join(blockers, ", "))
		return
	}

	// Extract details
	fields, ok := ticket["fields"].(map[string]interface{})
	if !ok {
		fmt.Fprintf(os.Stderr, "[%s] Error: Invalid ticket format (missing fields)\n", jiraTicketID)
		return
	}
	summary, _ := fields["summary"].(string)
	description := jClient.ParseDescription(ticket)

	// Epic Detection
	if parent, ok := fields["parent"].(map[string]interface{}); ok {
		if parentKey, ok := parent["key"].(string); ok {
			cfg.JiraEpicKey = parentKey
			fmt.Printf("[%s] Detected parent Epic: %s\n", jiraTicketID, cfg.JiraEpicKey)
		}
	}

	fmt.Printf("[%s] Ticket Found: %s\nSummary: %s\n", jiraTicketID, jiraTicketID, summary)

	// 3. Workspace Isolation (Create Temp Dir)
	timestamp := time.Now().Format("20060102-150405")
	pattern := fmt.Sprintf("recac-jira-%s-%s-*", jiraTicketID, timestamp)
	tempWorkspace, err := os.MkdirTemp("", pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s] Error creating temp workspace: %v\n", jiraTicketID, err)
		return
	}

	var repoURL string
	repoRegex := regexp.MustCompile(`(?i)Repo: (https?://\S+)`)
	matches := repoRegex.FindStringSubmatch(description)
	if len(matches) > 1 {
		repoURL = strings.TrimSuffix(matches[1], ".git")
		fmt.Printf("[%s] Found repository URL in ticket: %s\n", jiraTicketID, repoURL)

		if _, err := setupWorkspace(ctx, repoURL, tempWorkspace, jiraTicketID, cfg.JiraEpicKey, timestamp); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to setup workspace: %v\n", jiraTicketID, err)
		}
	}

	// 5. Create app_spec.txt
	specContent := fmt.Sprintf("# Jira Ticket: %s\n# Summary: %s\n\n%s", jiraTicketID, summary, description)
	specPath := filepath.Join(tempWorkspace, "app_spec.txt")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "[%s] Error writing app_spec.txt: %v\n", jiraTicketID, err)
		return
	}

	fmt.Printf("[%s] Workspace created: %s\n", jiraTicketID, tempWorkspace)

	// 5. Transition Ticket Status
	transition := viper.GetString("jira.transition")
	if transition == "" {
		transition = "In Progress"
	}

	fmt.Printf("[%s] Transitioning ticket to '%s'...\n", jiraTicketID, transition)
	if err := jClient.SmartTransition(ctx, jiraTicketID, transition); err != nil {
		fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to transition Jira ticket: %v\n", jiraTicketID, err)
	} else {
		fmt.Printf("[%s] Jira ticket status updated.\n", jiraTicketID)
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
		fmt.Fprintf(os.Stderr, "[%s] Session failed: %v\n", jiraTicketID, err)
	} else {
		fmt.Printf("[%s] Session completed successfully.\n", jiraTicketID)
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

		session, err := sm.StartSession(cfg.SessionName, command, projectPath)
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

		session := runner.NewSession(dockerCli, agentClient, projectPath, "recac-agent:latest", projectName, cfg.MaxAgents)
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

	dockerCli, err := docker.NewClient(projectName)
	if err != nil {
		return fmt.Errorf("failed to initialize Docker client: %v", err)
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	agentClient, err := getAgentClient(ctx, provider, model, projectPath, projectName)
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %v", err)
	}

	session := runner.NewSession(dockerCli, agentClient, projectPath, "recac-agent:latest", projectName, cfg.MaxAgents)
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
