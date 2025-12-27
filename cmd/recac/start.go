package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"recac/internal/agent"
	"recac/internal/docker"
	"recac/internal/runner"
	"recac/internal/telemetry"
	"recac/internal/ui"
)

func init() {
	startCmd.Flags().Bool("mock", false, "Start in mock mode (no Docker or API keys required)")
	startCmd.Flags().Bool("mock-docker", false, "Use mock docker client")
	startCmd.Flags().String("path", "", "Project path (skips wizard)")
	startCmd.Flags().Int("max-iterations", 20, "Maximum number of iterations")
	startCmd.Flags().Int("manager-frequency", 5, "Frequency of manager reviews")
	startCmd.Flags().Bool("detached", false, "Run session in background (detached mode)")
	startCmd.Flags().String("name", "", "Name for the session (required for detached mode)")
	viper.BindPFlag("mock", startCmd.Flags().Lookup("mock"))
	viper.BindPFlag("mock-docker", startCmd.Flags().Lookup("mock-docker"))
	viper.BindPFlag("path", startCmd.Flags().Lookup("path"))
	viper.BindPFlag("max_iterations", startCmd.Flags().Lookup("max-iterations"))
	viper.BindPFlag("manager_frequency", startCmd.Flags().Lookup("manager-frequency"))
	viper.BindPFlag("detached", startCmd.Flags().Lookup("detached"))
	viper.BindPFlag("name", startCmd.Flags().Lookup("name"))
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
		mockDocker := viper.GetBool("mock-docker")
		projectPath := viper.GetString("path")
		maxIterations := viper.GetInt("max_iterations")
		managerFrequency := viper.GetInt("manager_frequency")
		detached := viper.GetBool("detached")
		sessionName := viper.GetString("name")

		if debug {
			fmt.Println("Debug mode is enabled")
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
			if mockDocker {
				command = append(command, "--mock-docker")
			}
			if maxIterations != 20 {
				command = append(command, "--max-iterations", fmt.Sprintf("%d", maxIterations))
			}
			if managerFrequency != 5 {
				command = append(command, "--manager-frequency", fmt.Sprintf("%d", managerFrequency))
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
			session := runner.NewSession(dockerCli, agentClient, projectPath, "ubuntu:latest")
			session.MaxIterations = maxIterations
			session.ManagerFrequency = managerFrequency
			
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
		} else {
			fmt.Printf("Using project path from flag: %s\n", projectPath)
		}

		fmt.Println("\nStarting RECAC session...")

		// Start metrics server if telemetry is enabled
		metricsPort := viper.GetInt("metrics_port")
		if metricsPort == 0 {
			metricsPort = 9090 // Default port
		}
		metricsAddr := fmt.Sprintf(":%d", metricsPort)
		go func() {
			if err := telemetry.StartMetricsServer(metricsAddr); err != nil {
				telemetry.LogDebug("Metrics server error", "error", err)
			}
		}()

		// Initialize Docker Client
		var dockerCli *docker.Client
		if mockDocker {
			dockerCli, _ = docker.NewMockClient()
		} else {
			var err error
			dockerCli, err = docker.NewClient()
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
				}
			}
		}

		model := viper.GetString("model")
		if model == "" {
			if provider == "gemini" {
				model = "gemini-pro"
			} else if provider == "openai" {
				model = "gpt-4"
			} else if provider == "ollama" {
				model = "llama2" // Default Ollama model
			}
		}
		
		// For Ollama, apiKey (baseURL) can be empty (defaults to localhost:11434)
		// For other providers, require apiKey
		if apiKey == "" && provider != "ollama" {
			apiKey = "dummy-key" // Allow starting without key (will fail on Send)
		}

		agentClient, err := agent.NewAgent(provider, apiKey, model)
		if err != nil {
			fmt.Printf("Failed to initialize agent: %v\n", err)
			os.Exit(1)
		}

		// Start Session
		session := runner.NewSession(dockerCli, agentClient, projectPath, "ubuntu:latest")
		session.MaxIterations = maxIterations
		session.ManagerFrequency = managerFrequency

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