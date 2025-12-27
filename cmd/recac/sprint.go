package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	tea "github.com/charmbracelet/bubbletea"
	"recac/internal/runner"
	"recac/internal/telemetry"
	"recac/internal/ui"
)

var workers int
var slackWebhook string

func init() {
	sprintCmd.Flags().IntVarP(&workers, "workers", "w", 3, "Number of concurrent workers")
	sprintCmd.Flags().StringVar(&slackWebhook, "slack-webhook", "", "Slack Webhook URL for notifications")
	rootCmd.AddCommand(sprintCmd)
}

var sprintCmd = &cobra.Command{
	Use:   "sprint",
	Short: "Run multiple agents in parallel (Sprint Mode)",
	Long:  `Sprint Mode decomposes the app spec into independent tasks and executes them in parallel using a worker pool.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Panic recovery for graceful shutdown
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "\n=== CRITICAL ERROR: Sprint Panic ===\n")
				fmt.Fprintf(os.Stderr, "Error: %v\n", r)
				fmt.Fprintf(os.Stderr, "Attempting graceful shutdown...\n")
				os.Exit(1)
			}
		}()

		telemetry.LogDebug("Starting sprint command", "workers", workers)

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

		// Initialize and start worker pool
		pool := runner.NewWorkerPool(workers)
		pool.Start()
		defer pool.Stop()

		// Give workers a moment to log their startup messages
		time.Sleep(100 * time.Millisecond)

		// Check if we have a TTY before launching TUI
		hasTTY := false
		if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
			tty.Close()
			hasTTY = true
		}

		if !hasTTY {
			// No TTY available, just keep the pool running for a bit to demonstrate workers
			fmt.Println("No TTY available, worker pool is running. Press Ctrl+C to exit.")
			// Wait for interrupt signal
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			<-sigChan
			fmt.Println("\nShutting down...")
			return
		}

		// Launch TUI
		board := ui.NewSprintBoardModel()
		
		// Add mock tasks for demonstration
		mockTasks := []string{
			"Implement user authentication",
			"Add database migrations",
			"Create API endpoints",
			"Write unit tests",
			"Setup CI/CD pipeline",
		}
		
		for i, taskName := range mockTasks {
			board.AddTask(fmt.Sprintf("task-%d", i), taskName)
		}

		// Start the TUI program
		program := tea.NewProgram(board, tea.WithAltScreen())
		
		// In a real implementation, this would be integrated with the worker pool
		// For now, we'll simulate task movement in a goroutine
		go func() {
			time.Sleep(1 * time.Second)
			// Move first task to In Progress
			program.Send(ui.SprintBoardMsg{TaskID: "task-0", Status: ui.TaskInProgress})
			
			time.Sleep(2 * time.Second)
			// Move second task to In Progress
			program.Send(ui.SprintBoardMsg{TaskID: "task-1", Status: ui.TaskInProgress})
			
			time.Sleep(1 * time.Second)
			// Move first task to Done
			program.Send(ui.SprintBoardMsg{TaskID: "task-0", Status: ui.TaskDone})
			
			time.Sleep(2 * time.Second)
			// Move third task to In Progress
			program.Send(ui.SprintBoardMsg{TaskID: "task-2", Status: ui.TaskInProgress})
			
			time.Sleep(1 * time.Second)
			// Move second task to Done
			program.Send(ui.SprintBoardMsg{TaskID: "task-1", Status: ui.TaskDone})
		}()

		if _, err := program.Run(); err != nil {
			// TUI error is not fatal - workers are still running
			fmt.Fprintf(os.Stderr, "Warning: TUI error: %v (workers continue running)\n", err)
		}
	},
}
