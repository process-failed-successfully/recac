package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
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
	startCmd.Flags().Bool("mock", false, "Start in mock mode with dashboard")
	startCmd.Flags().Bool("mock-docker", false, "Use mock docker client")
	startCmd.Flags().String("path", "", "Project path (skips wizard)")
	viper.BindPFlag("mock", startCmd.Flags().Lookup("mock"))
	viper.BindPFlag("mock-docker", startCmd.Flags().Lookup("mock-docker"))
	viper.BindPFlag("path", startCmd.Flags().Lookup("path"))
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

		if debug {
			fmt.Println("Debug mode is enabled")
		}

		if isMock {
			p := tea.NewProgram(ui.NewDashboardModel(), tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				fmt.Printf("Error starting dashboard: %v", err)
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
				}
			}
		}

		model := viper.GetString("model")
		if model == "" {
			if provider == "gemini" {
				model = "gemini-pro"
			} else if provider == "openai" {
				model = "gpt-4"
			}
		}
		
		if apiKey == "" {
			apiKey = "dummy-key" // Allow starting without key (will fail on Send)
		}

		agentClient, err := agent.NewAgent(provider, apiKey, model)
		if err != nil {
			fmt.Printf("Failed to initialize agent: %v\n", err)
			os.Exit(1)
		}

		// Start Session
		session := runner.NewSession(dockerCli, agentClient, projectPath, "ubuntu:latest")
		if err := session.Start(ctx); err != nil {
			if ctx.Err() != nil {
				fmt.Println("\nSession interrupted by user.")
				return
			}
			fmt.Printf("Session failed: %v\n", err)
			os.Exit(1)
		}
	},
}