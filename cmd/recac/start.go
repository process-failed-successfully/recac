package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"recac/internal/cmdutils"
	"recac/internal/jira"
	"recac/internal/ui"
	"recac/internal/workflow"

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
	startCmd.Flags().String("resume-from", "", "Resume from a specific workspace path")
	startCmd.Flags().MarkHidden("resume-from")

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
		if cmd.Flags().Changed("detached") {
			detached, _ = cmd.Flags().GetBool("detached")
		}

		sessionName := viper.GetString("name")
		if cmd.Flags().Changed("name") {
			sessionName, _ = cmd.Flags().GetString("name")
		}

		jiraTicketID := viper.GetString("jira")
		if cmd.Flags().Changed("jira") {
			jiraTicketID, _ = cmd.Flags().GetString("jira")
		}

		// Handle Jira Ticket Workflow
		jiraLabel := viper.GetString("jira_label")

		// Persistent Flags used in config
		autoMergeFlag, _ := cmd.Flags().GetBool("auto-merge")
		skipQAFlag, _ := cmd.Flags().GetBool("skip-qa")

		repoURL, _ := cmd.Flags().GetString("repo-url")
		summary, _ := cmd.Flags().GetString("summary")
		description, _ := cmd.Flags().GetString("description")

		// Global Configuration (using workflow.SessionConfig)
		cfg := workflow.SessionConfig{
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
			if err := workflow.RunWorkflow(ctx, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Resumed session failed: %v\n", err)
				exit(1)
			}
			return
		}

		if repoURL != "" {
			workflow.ProcessDirectTask(ctx, cfg)
			return
		}

		if jiraTicketID != "" || jiraLabel != "" {
			jClient, err := cmdutils.GetJiraClient(ctx)
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
				workflow.ProcessJiraTicket(ctx, ticketIDs[0], jClient, cfg, nil)
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
					workflow.ProcessJiraTicket(ctx, targetID, jClient, cfg, localTickets)

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

		if err := workflow.RunWorkflow(ctx, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Session failed: %v\n", err)
			exit(1)
		}
	},
}
