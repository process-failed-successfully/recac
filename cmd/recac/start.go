package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"recac/internal/agent"
	"recac/internal/docker"
	"recac/internal/jira"
	"recac/internal/runner"
	"recac/internal/telemetry"
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
				os.Exit(1)
			}
		}()

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		debug := viper.GetBool("debug")
		isMock := viper.GetBool("mock")
		projectPath := viper.GetString("path")
		maxIterations := viper.GetInt("max_iterations")
		managerFrequency := viper.GetInt("manager_frequency")
		maxAgents := viper.GetInt("max_agents")
		taskMaxIterations := viper.GetInt("task_max_iterations")
		detached := viper.GetBool("detached")
		sessionName := viper.GetString("name")
		jiraTicketID := viper.GetString("jira")

		if debug {
			fmt.Println("Debug mode is enabled")
		}

		// Handle Jira Ticket Workflow
		jiraLabel := viper.GetString("jira_label")

		if jiraTicketID != "" || jiraLabel != "" {
			// 1. Validate Credentials
			jiraURL := viper.GetString("jira.url")
			jiraEmail := viper.GetString("jira.username")
			jiraToken := viper.GetString("jira.api_token")

			if jiraURL == "" || jiraEmail == "" || jiraToken == "" {
				// Fallback to env vars
				if envURL := os.Getenv("JIRA_URL"); envURL != "" {
					jiraURL = envURL
				}
				if envEmail := os.Getenv("JIRA_USERNAME"); envEmail != "" {
					jiraEmail = envEmail
				} else if envEmail := os.Getenv("JIRA_EMAIL"); envEmail != "" {
					// Support JIRA_EMAIL as alias for JIRA_USERNAME
					jiraEmail = envEmail
				}
				if envToken := os.Getenv("JIRA_API_TOKEN"); envToken != "" {
					jiraToken = envToken
				}

				if jiraURL == "" || jiraEmail == "" || jiraToken == "" {
					fmt.Fprintln(os.Stderr, "Error: Jira credentials not found. Please set JIRA_URL, JIRA_USERNAME/JIRA_EMAIL, and JIRA_API_TOKEN environment variables or configure 'jira' section in config.yaml.")
					os.Exit(1)
				}
			}

			jClient := jira.NewClient(jiraURL, jiraEmail, jiraToken)

			// If Label is provided but not ID, search for ticket
			if jiraTicketID == "" && jiraLabel != "" {
				fmt.Printf("Searching for tickets with label '%s'...\n", jiraLabel)
				// Search for issues with label AND status not done
				jql := fmt.Sprintf("labels = \"%s\" AND statusCategory != Done ORDER BY created DESC", jiraLabel)
				issues, err := jClient.SearchIssues(ctx, jql)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error searching Jira tickets: %v\n", err)
					os.Exit(1)
				}

				if len(issues) == 0 {
					fmt.Printf("No open tickets found with label '%s'. Exiting.\n", jiraLabel)
					return
				}

				// Pick the first one
				// TODO: Handle max-parallel-tickets logic here if we want to run multiple
				// For now, we grab the first one to restore basic functionality
				firstIssue := issues[0]
				jiraTicketID, _ = firstIssue["key"].(string)
				fmt.Printf("Found %d tickets. Selected: %s\n", len(issues), jiraTicketID)
			}

			// Proceed with Single Ticket Workflow
			fmt.Printf("Initializing session from Jira Ticket: %s\n", jiraTicketID)

			// 2. Fetch Ticket (if we searched, we technically have it, but consistent flow is safer)
			ticket, err := jClient.GetTicket(ctx, jiraTicketID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error fetching Jira ticket: %v\n", err)
				os.Exit(1)
			}

			// Extract details
			fields, ok := ticket["fields"].(map[string]interface{})
			if !ok {
				fmt.Fprintln(os.Stderr, "Error: Invalid ticket format (missing fields)")
				os.Exit(1)
			}
			summary, _ := fields["summary"].(string)
			description := jClient.ParseDescription(ticket)

			fmt.Printf("Ticket Found: %s\nSummary: %s\n", jiraTicketID, summary)

			// 3. Workspace Isolation (Create Temp Dir)
			timestamp := time.Now().Format("20060102-150405")
			tempWorkspace := filepath.Join(os.TempDir(), fmt.Sprintf("recac-jira-%s-%s", jiraTicketID, timestamp))

			if err := os.MkdirAll(tempWorkspace, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating temp workspace: %v\n", err)
				os.Exit(1)
			}

			// 4. Create app_spec.txt
			specContent := fmt.Sprintf("# Jira Ticket: %s\n# Summary: %s\n\n%s", jiraTicketID, summary, description)
			specPath := filepath.Join(tempWorkspace, "app_spec.txt")
			if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing app_spec.txt: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Workspace created: %s\n", tempWorkspace)

			// 5. Transition Ticket Status (Status Sync)
			transition := viper.GetString("jira.transition")
			if transition == "" {
				transition = "In Progress" // Smart default
			}

			fmt.Printf("Transitioning ticket %s to '%s'...\n", jiraTicketID, transition)
			if err := jClient.SmartTransition(ctx, jiraTicketID, transition); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to transition Jira ticket: %v\n", err)
			} else {
				fmt.Println("Jira ticket status updated.")
			}

			// Override projectPath
			projectPath = tempWorkspace

			// If session name is not set, set it to Ticket ID
			if sessionName == "" {
				sessionName = jiraTicketID
				// Also update viper for consistency if needed downstream
				viper.Set("name", sessionName)
			}
		}

		// Handle detached mode
		if detached {
			if sessionName == "" {
				fmt.Fprintf(os.Stderr, "Error: --name is required when using --detached\n")
				os.Exit(1)
			}

			// Get the executable path
			executable, err := os.Executable()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to get executable path: %v\n", err)
				os.Exit(1)
			}

			// Resolve absolute path and symlinks
			executable, err = filepath.EvalSymlinks(executable)
			if err != nil {
				// If symlink resolution fails, try absolute path
				executable, err = filepath.Abs(executable)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: failed to resolve executable path: %v\n", err)
					os.Exit(1)
				}
			} else {
				// EvalSymlinks already returns absolute path, but ensure it
				executable, err = filepath.Abs(executable)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: failed to get absolute path: %v\n", err)
					os.Exit(1)
				}
			}

			// Verify executable exists and is accessible
			if stat, err := os.Stat(executable); err != nil {
				// Try fallback: look for recac-app in current directory
				cwd, _ := os.Getwd()
				fallback := filepath.Join(cwd, "recac-app")
				if stat2, err2 := os.Stat(fallback); err2 == nil {
					executable = fallback
					stat = stat2
				} else {
					fmt.Fprintf(os.Stderr, "Error: executable not found at %s: %v\n", executable, err)
					os.Exit(1)
				}
			} else {
				// Verify it's executable
				if stat.Mode()&0111 == 0 {
					fmt.Fprintf(os.Stderr, "Error: %s is not executable\n", executable)
					os.Exit(1)
				}
			}

			// Build command to re-execute in foreground (without --detached)
			command := []string{executable, "start"}
			if projectPath != "" {
				command = append(command, "--path", projectPath)
			}
			if isMock {
				command = append(command, "--mock")
			}
			if maxIterations != 20 {
				command = append(command, "--max-iterations", fmt.Sprintf("%d", maxIterations))
			}
			if managerFrequency != 5 {
				command = append(command, "--manager-frequency", fmt.Sprintf("%d", managerFrequency))
			}
			if taskMaxIterations != 10 {
				command = append(command, "--task-max-iterations", fmt.Sprintf("%d", taskMaxIterations))
			}
			// Pass allow-dirty flag if set
			allowDirty := viper.GetBool("allow_dirty")
			if allowDirty {
				command = append(command, "--allow-dirty")
			}

			// Use default workspace if not provided
			if projectPath == "" {
				projectPath = "."
			}

			// Start session in background
			sm, err := runner.NewSessionManager()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to create session manager: %v\n", err)
				os.Exit(1)
			}

			session, err := sm.StartSession(sessionName, command, projectPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to start detached session: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Session '%s' started in background (PID: %d)\n", sessionName, session.PID)
			fmt.Printf("Log file: %s\n", session.LogFile)
			fmt.Printf("Use 'recac-app list' to view sessions\n")
			fmt.Printf("Use 'recac-app logs %s' to view output\n", sessionName)
			return
		}

		// Mock mode: start session with mock Docker and mock agent
		if isMock {
			fmt.Println("Starting in MOCK MODE (no Docker or API keys required)")

			// Use mock Docker client
			dockerCli, _ := docker.NewMockClient()

			// Use mock agent
			var agentClient agent.Agent = agent.NewMockAgent()

			// Default project path if not provided
			if projectPath == "" {
				projectPath = "/tmp/recac-mock-workspace"
				fmt.Printf("No project path provided, using: %s\n", projectPath)
			}

			// Start Session
			// Mock project name
			mockProject := "mock-project"
			session := runner.NewSession(dockerCli, agentClient, projectPath, "recac-agent:latest", mockProject, maxAgents)
			session.MaxIterations = maxIterations
			session.TaskMaxIterations = taskMaxIterations
			session.ManagerFrequency = managerFrequency
			session.StreamOutput = viper.GetBool("stream")

			// Configure StateManager for agents that support it (e.g., Gemini)
			if session.StateManager != nil {
				// Type assert to check if agent supports StateManager
				if geminiClient, ok := agentClient.(*agent.GeminiClient); ok {
					geminiClient.WithStateManager(session.StateManager)
				}
			}

			if err := session.Start(ctx); err != nil {
				if ctx.Err() != nil {
					fmt.Println("\nSession interrupted by user.")
					return
				}
				fmt.Printf("Session initialization failed: %v\n", err)
				os.Exit(1)
			}

			// Run Autonomous Loop
			if err := session.RunLoop(ctx); err != nil {
				if ctx.Err() != nil {
					fmt.Println("\nSession interrupted by user.")
					return
				}
				fmt.Printf("Session loop failed: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// TUI Wizard (only if path is not provided)
		if projectPath == "" {
			p := tea.NewProgram(ui.NewWizardModel())
			m, err := p.Run()
			if err != nil {
				fmt.Printf("Alas, there's been an error: %v", err)
				os.Exit(1)
			}

			// Cast model to WizardModel to retrieve data
			wizardModel, ok := m.(ui.WizardModel)
			if !ok {
				fmt.Println("Could not retrieve wizard data")
				os.Exit(1)
			}
			projectPath = wizardModel.Path
			if projectPath == "" {
				fmt.Println("No project path selected. Exiting.")
				return
			}

			// Set Provider and MaxAgents from Wizard if available
			if wizardModel.Provider != "" {
				viper.Set("provider", wizardModel.Provider)
				fmt.Printf("Using provider: %s\n", wizardModel.Provider)
			}
			if wizardModel.MaxAgents > 0 {
				maxAgents = wizardModel.MaxAgents
				viper.Set("max_agents", maxAgents)
				fmt.Printf("Max parallel agents: %d\n", maxAgents)
			}
			if wizardModel.TaskMaxIterations > 0 {
				taskMaxIterations = wizardModel.TaskMaxIterations
				viper.Set("task_max_iterations", taskMaxIterations)
				fmt.Printf("Max task iterations: %d\n", taskMaxIterations)
			}
		} else {
			fmt.Printf("Using project path from flag: %s\n", projectPath)
		}

		fmt.Println("\nStarting RECAC session...")

		// Pre-flight Check: Git Clean Status
		allowDirty := viper.GetBool("allow_dirty")
		if !allowDirty && !isMock {
			// Check if projectPath is inside a git repository
			cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
			cmd.Dir = projectPath
			if err := cmd.Run(); err == nil {
				// It is a git repo, check status
				cmd := exec.Command("git", "status", "--porcelain")
				cmd.Dir = projectPath
				output, err := cmd.Output()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to check git status: %v\n", err)
				} else if len(output) > 0 {
					fmt.Fprintf(os.Stderr, "Error: Uncommitted changes detected in %s\n", projectPath)
					fmt.Fprintf(os.Stderr, "Run with --allow-dirty to bypass this check.\n")
					os.Exit(1)
				}
			}
		}

		// Start metrics server if telemetry is enabled
		metricsPort := viper.GetInt("metrics_port")
		if metricsPort == 0 {
			metricsPort = 9090 // Default port
		}
		go func() {
			if err := telemetry.StartMetricsServer(metricsPort); err != nil {
				telemetry.LogDebug("Metrics server error", "error", err)
			}
		}()

		// Initialize Docker Client
		var dockerCli *docker.Client

		// Derive project name from path
		projectName := filepath.Base(projectPath)
		if projectName == "." || projectName == "/" {
			cwd, _ := os.Getwd()
			projectName = filepath.Base(cwd)
		}

		{
			var err error
			dockerCli, err = docker.NewClient(projectName)
			if err != nil {
				fmt.Printf("Failed to initialize Docker client: %v\n", err)
				os.Exit(1)
			}
		}

		// Initialize Agent
		provider := viper.GetString("provider")
		if provider == "" {
			provider = "gemini" // Default
		}

		apiKey := viper.GetString("api_key")
		if apiKey == "" {
			apiKey = os.Getenv("API_KEY")
			if apiKey == "" {
				// Fallback to provider-specific env vars
				if provider == "gemini" {
					apiKey = os.Getenv("GEMINI_API_KEY")
				} else if provider == "openai" {
					apiKey = os.Getenv("OPENAI_API_KEY")
				} else if provider == "ollama" {
					// For Ollama, apiKey is actually baseURL (optional)
					apiKey = os.Getenv("OLLAMA_BASE_URL")
				} else if provider == "openrouter" {
					apiKey = os.Getenv("OPENROUTER_API_KEY")
				}
			}
		}

		// Determine model
		// Priority: --model (flag) > RECAC_MODEL (env) > Provider default > Config
		// Note: viper.GetString("model") returns the flag value if set,
		// otherwise it falls back to config.
		model := viper.GetString("model")
		envModel := os.Getenv("RECAC_MODEL")

		// If the flag was NOT set (empty), check if env var exists
		// If flag IS set, viper.GetString already has it.
		// However, to be explicit about priority:
		if !cmd.Flags().Changed("model") && envModel != "" {
			model = envModel
		}

		if model == "" {
			// If no explicit flag or env, verify/override defaults for specific providers
			if provider == "gemini-cli" {
				model = "auto"
			} else if provider == "cursor-cli" {
				model = "auto"
			} else if provider == "openrouter" {
				model = "deepseek/deepseek-v3.2"
			}
			// For other providers (gemini, openai, ollama), fallback to viper (config) or defaults
			if model == "" {
				if provider == "gemini" {
					model = "gemini-pro"
				} else if provider == "openai" {
					model = "gpt-4"
				} else if provider == "ollama" {
					model = "llama2"
				}
			}
		}

		// For Ollama, apiKey (baseURL) can be empty (defaults to localhost:11434)
		// For other providers, require apiKey
		if apiKey == "" && provider != "ollama" {
			apiKey = "dummy-key" // Allow starting without key (will fail on Send)
		}

		agentClient, err := agent.NewAgent(provider, apiKey, model, projectPath, projectName)
		if err != nil {
			fmt.Printf("Failed to initialize agent: %v\n", err)
			os.Exit(1)
		}

		// Start Session
		session := runner.NewSession(dockerCli, agentClient, projectPath, "recac-agent:latest", projectName, maxAgents)
		session.MaxIterations = maxIterations
		session.TaskMaxIterations = taskMaxIterations
		session.ManagerFrequency = managerFrequency
		session.ManagerFirst = viper.GetBool("manager_first")
		session.StreamOutput = viper.GetBool("stream")
		session.AutoMerge = viper.GetBool("auto_merge")

		// Configure StateManager for agents that support it (Gemini, OpenAI)
		if session.StateManager != nil {
			// Read max_tokens from config (agent.max_tokens)
			maxTokens := viper.GetInt("agent.max_tokens")
			if maxTokens == 0 {
				// Try alternative config path
				maxTokens = viper.GetInt("max_tokens")
			}
			if maxTokens == 0 {
				// Default based on provider
				if provider == "gemini" {
					maxTokens = 32000
				} else if provider == "openai" {
					maxTokens = 128000
				} else if provider == "openrouter" {
					maxTokens = 128000
				}
			}

			// Initialize state with max_tokens
			if err := session.InitializeAgentState(maxTokens); err != nil {
				fmt.Printf("Warning: Failed to initialize agent state with max_tokens: %v\n", err)
			}

			// Type assert to check if agent supports StateManager
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
				fmt.Println("\nSession interrupted by user.")
				return
			}
			fmt.Printf("Session initialization failed: %v\n", err)
			os.Exit(1)
		}

		// Run Autonomous Loop
		if err := session.RunLoop(ctx); err != nil {
			if ctx.Err() != nil {
				fmt.Println("\nSession interrupted by user.")
				return
			}
			fmt.Printf("Session loop failed: %v\n", err)
			os.Exit(1)
		}
	},
}
