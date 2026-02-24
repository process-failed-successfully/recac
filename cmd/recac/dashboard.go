package main

import (
	"fmt"
	"net/http"
	"time"

	"recac/internal/tui"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Launch the Orchestrator TUI Dashboard",
	Long:  `Connects to a running orchestrator instance and displays a real-time dashboard of agents, jobs, and logs.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		host := viper.GetString("orchestrator.host")

		// Check connectivity
		fmt.Fprintf(cmd.OutOrStdout(), "Connecting to orchestrator at %s...\n", host)
		client := http.Client{
			Timeout: 2 * time.Second,
		}
		resp, err := client.Get(fmt.Sprintf("%s/status", host))
		if err != nil {
			return fmt.Errorf("failed to connect to orchestrator at %s: %w\n\nMake sure the orchestrator is running:\n  recac orchestrate --mode local", host, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("orchestrator returned status %d", resp.StatusCode)
		}

		// Launch TUI
		if err := tui.StartDashboard(host); err != nil {
			return fmt.Errorf("dashboard failed: %w", err)
		}
		return nil
	},
}

func init() {
	dashboardCmd.Flags().String("host", "http://localhost:2112", "Orchestrator host URL")
	viper.BindPFlag("orchestrator.host", dashboardCmd.Flags().Lookup("host"))
	viper.BindEnv("orchestrator.host", "RECAC_ORCHESTRATOR_HOST")

	rootCmd.AddCommand(dashboardCmd)
}
