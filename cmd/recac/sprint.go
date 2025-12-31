package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"recac/internal/runner"
	"recac/internal/telemetry"
	"recac/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var workers int
var slackWebhook string
var dependencyAware bool
var featureListPath string

func init() {
	sprintCmd.Flags().IntVarP(&workers, "workers", "w", 3, "Number of concurrent workers")
	sprintCmd.Flags().StringVar(&slackWebhook, "slack-webhook", "", "Slack Webhook URL for notifications")
	sprintCmd.Flags().BoolVar(&dependencyAware, "dependency-aware", false, "Enable dependency-aware task execution")
	sprintCmd.Flags().StringVar(&featureListPath, "feature-list", "feature_list.json", "Path to feature_list.json file")
	sprintCmd.Flags().String("model", "", "Model to use (overrides config and RECAC_MODEL env var)")
	viper.BindPFlag("model", sprintCmd.Flags().Lookup("model"))
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
			// No TTY available
			if dependencyAware {
				// Dependency-aware mode: load task graph and execute with dependencies
				fmt.Println("Dependency-aware mode enabled. Loading task graph...")

				graph := runner.NewTaskGraph()

				// Try to load from feature_list.json
				if err := graph.LoadFromFeatureList(featureListPath); err != nil {
					fmt.Printf("Warning: Failed to load feature_list.json: %v\n", err)
					fmt.Println("Creating test task graph with dependencies...")

					// Create test tasks with dependencies for demonstration
					graph.AddNode("task-0", "Task 0 (no dependencies)", []string{})
					graph.AddNode("task-1", "Task 1 (depends on task-0)", []string{"task-0"})
					graph.AddNode("task-2", "Task 2 (depends on task-0)", []string{"task-0"})
					graph.AddNode("task-3", "Task 3 (depends on task-1, task-2)", []string{"task-1", "task-2"})
					graph.AddNode("task-4", "Task 4 (no dependencies)", []string{})
					graph.AddNode("task-5", "Task 5 (depends on task-4)", []string{"task-4"})
				}

				// Check for circular dependencies
				if cycle, err := graph.DetectCycles(); err != nil {
					fmt.Printf("Error: %v\n", err)
					os.Exit(1)
				} else if cycle != nil {
					fmt.Printf("Error: Circular dependency detected: %v\n", cycle)
					os.Exit(1)
				}

				// Get topological sort
				executionOrder, err := graph.TopologicalSort()
				if err != nil {
					fmt.Printf("Error determining execution order: %v\n", err)
					os.Exit(1)
				}

				fmt.Printf("Task execution order (topological sort): %v\n", executionOrder)

				// Create dependency executor
				executor := runner.NewDependencyExecutor(graph, pool)

				// Register task functions
				for _, taskID := range executionOrder {
					taskID := taskID // Capture for closure
					node, _ := graph.GetTask(taskID)
					executor.RegisterTask(taskID, func(workerID int) error {
						fmt.Printf("Task %s (%s): Executing on Worker %d\n", taskID, node.Name, workerID)
						time.Sleep(200 * time.Millisecond) // Simulate work
						return nil
					})
				}

				// Execute with dependency awareness
				fmt.Println("\nExecuting tasks with dependency awareness...")
				if err := executor.Execute(); err != nil {
					fmt.Printf("Error during execution: %v\n", err)
					os.Exit(1)
				}

				// Print summary
				summary := graph.GetTaskSummary()
				fmt.Printf("\n=== Execution Summary ===\n")
				for status, count := range summary {
					fmt.Printf("%s: %d\n", status, count)
				}

				fmt.Println("\nAll tasks completed successfully.")
				return
			} else {
				// Standard mode: submit test tasks to demonstrate worker pool functionality
				fmt.Println("No TTY available, submitting test tasks to demonstrate worker pool...")

				// Submit multiple tasks to demonstrate distribution across workers
				numTestTasks := workers * 2 // Submit 2 tasks per worker to show distribution
				var taskCounter int
				var mu sync.Mutex

				fmt.Printf("Submitting %d tasks to %d workers...\n", numTestTasks, workers)

				for i := 0; i < numTestTasks; i++ {
					taskNum := i
					pool.Submit(func(workerID int) error {
						mu.Lock()
						taskCounter++
						currentCount := taskCounter
						mu.Unlock()

						fmt.Printf("Task %d: Executed by Worker %d (task %d/%d)\n", taskNum, workerID, currentCount, numTestTasks)
						// Simulate work with a small delay
						time.Sleep(100 * time.Millisecond)
						return nil
					})
				}

				// Wait a moment for tasks to complete
				time.Sleep(time.Duration(numTestTasks*100+500) * time.Millisecond)

				fmt.Printf("\nAll tasks completed. Worker pool is running. Press Ctrl+C to exit.\n")
				// Wait for interrupt signal
				sigChan := make(chan os.Signal, 1)
				signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
				<-sigChan
				fmt.Println("\nShutting down...")
				return
			}
		}

		// Launch TUI
		board := ui.NewSprintBoardModel()

		// Try to load from feature_list.json
		graph := runner.NewTaskGraph()
		if err := graph.LoadFromFeatureList(featureListPath); err == nil {
			telemetry.LogDebug("Loaded features for TUI", "count", len(graph.Nodes))
			for id, node := range graph.Nodes {
				board.AddTask(id, node.Name)
			}
		} else {
			// Fallback to mock tasks for demonstration if file missing
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
