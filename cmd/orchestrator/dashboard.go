package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"recac/internal/model"
	"recac/internal/runner"
	"recac/internal/telemetry"
	"recac/internal/ui"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "View the real-time status of agent jobs",
	Long:  "Connects to the configured backend (Kubernetes or Docker) and displays a TUI dashboard of active agent jobs.",
	RunE:  runDashboard,
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
	// Dashboard specific flags
	dashboardCmd.Flags().Bool("show-costs", false, "Show estimated costs (if available)")
	dashboardCmd.Flags().String("sort", "time", "Sort by: time, name, cost")
}

func runDashboard(cmd *cobra.Command, args []string) error {
	mode := viper.GetString("orchestrator.mode")
	showCosts, _ := cmd.Flags().GetBool("show-costs")
	sortBy, _ := cmd.Flags().GetString("sort")

	logger := telemetry.NewLogger(viper.GetBool("verbose"), "dashboard", false)
	logger.Info("Starting Dashboard", "mode", mode)

	fetcher := func() ([]model.UnifiedSession, error) {
		// Create a context with timeout for fetching
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if mode == "k8s" || mode == "kubernetes" {
			return fetchK8sSessions(ctx)
		}
		// Default to local/docker which uses the SessionManager
		return fetchLocalSessions(ctx)
	}

	return ui.StartPsDashboard(fetcher, showCosts, sortBy)
}

func fetchK8sSessions(ctx context.Context) ([]model.UnifiedSession, error) {
	// Initialize K8s Client
	config, err := rest.InClusterConfig()
	if err != nil {
		var kubeconfig string
		if os.Getenv("KUBECONFIG") != "" {
			kubeconfig = os.Getenv("KUBECONFIG")
		} else if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	namespace := viper.GetString("orchestrator.namespace")
	if namespace == "" {
		namespace = "default"
	}

	// List Jobs with label app=recac-agent
	jobs, err := clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=recac-agent",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	var sessions []model.UnifiedSession
	for _, job := range jobs.Items {
		name := job.Name
		status := "Unknown"
		if job.Status.Active > 0 {
			status = "Running"
		} else if job.Status.Succeeded > 0 {
			status = "Completed"
		} else if job.Status.Failed > 0 {
			status = "Failed"
		}

		startTime := job.CreationTimestamp.Time
		completionTime := time.Time{}
		if job.Status.CompletionTime != nil {
			completionTime = job.Status.CompletionTime.Time
		}

		// Try to get ticket ID from label
		ticketID := job.Labels["ticket"]
		if ticketID == "" {
			ticketID = "N/A"
		}

		sessions = append(sessions, model.UnifiedSession{
			Name:         name,
			Status:       status,
			StartTime:    startTime,
			LastActivity: startTime, // Approximation as we don't stream logs here
			EndTime:      completionTime,
			Location:     "k8s",
			Goal:         fmt.Sprintf("Ticket: %s", ticketID),
			CPU:          "N/A",
			Memory:       "N/A",
		})
	}
	return sessions, nil
}

func fetchLocalSessions(ctx context.Context) ([]model.UnifiedSession, error) {
	sm, err := runner.NewSessionManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	sessions, err := sm.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	var unified []model.UnifiedSession
	for _, s := range sessions {
		// Only show sessions managed by orchestrator or relevant ones
		// s.Type should be "orchestrated-docker" or "docker"

		// Parse Goal from Command if possible
		goal := "Unknown"
		if len(s.Command) > 0 {
			goal = strings.Join(s.Command, " ")
		}

		unified = append(unified, model.UnifiedSession{
			Name:         s.Name,
			Status:       s.Status,
			StartTime:    s.StartTime,
			LastActivity: s.StartTime, // SessionState doesn't track last activity
			EndTime:      s.EndTime,
			Location:     "local",
			Goal:         goal,
			CPU:          "N/A",
			Memory:       "N/A",
		})
	}
	return unified, nil
}
